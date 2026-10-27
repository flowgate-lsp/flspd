package services

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/flowgate-lsp/flspd/core"
	"github.com/flowgate-lsp/flspd/platform/config"
	"github.com/flowgate-lsp/flspd/ports"
	"github.com/nbd-wtf/go-nostr"
	"github.com/ohstr/nmilat/nip01"
	"github.com/ohstr/nmilat/niplsp"
	"github.com/rs/zerolog/log"
)

// Publisher defaults (NIP-LSP §Publishing: republish at least every 7 days,
// expiration at most 30 days after created_at; niplsp sets it to exactly 30
// days when Meta.Expiration is 0).
const (
	DefaultAnnouncePollInterval   = 30 * time.Second
	DefaultAnnounceRepublishEvery = 7 * 24 * time.Hour
	DefaultAnnouncePublishTimeout = 15 * time.Second
)

// AnnounceNode is one node the publisher announces.
type AnnounceNode struct {
	Node    *core.Node
	Signer  ports.AnnouncementSigner
	Answers ports.LSPSAnswerSource
	// Chain and Network are the node's, from flnd GetInfo (n, network).
	Chain   string
	Network string
}

// RelayPublisher sends an event to one relay and waits for its answer.
type RelayPublisher interface {
	Publish(ctx context.Context, relayURL string, ev nostr.Event) error
}

// AnnounceServiceConfig configures the publisher. Zero durations take the
// defaults above; Now defaults to time.Now.
type AnnounceServiceConfig struct {
	Relays         []string
	Store          ports.AnnounceStateStore
	Publisher      RelayPublisher
	PollInterval   time.Duration
	RepublishEvery time.Duration
	PublishTimeout time.Duration
	Now            func() time.Time
}

// AnnounceService publishes each announcing node's NIP-LSP announcement
// (T8-3) and checks every published event against the live answers (T8-4).
//
// On each run (at start, every PollInterval, and on Trigger) it builds the
// node's Fields from the live LSPS answers, the announce block and the
// stored status, and publishes when nothing was published yet, when any
// announced value differs from the last publication (a hot-swap, a
// Pause/Resume/Unlist, a JIT switch), or when RepublishEvery has passed.
// A closed status is published once, then nothing more until a Resume. A
// failed self-check blocks the node's publishing until Recheck.
type AnnounceService struct {
	cfg     AnnounceServiceConfig
	trigger chan struct{}
	runMu   sync.Mutex // one run at a time
	mu      sync.Mutex // guards each node's status

	nodesMu sync.RWMutex
	nodes   []*announceNodeState
}

type announceNodeState struct {
	AnnounceNode
	fingerprint string // of the last published Fields
	lastAt      time.Time
	status      core.AnnouncePublisherStatus
}

// NewAnnounceService returns a publisher for nodes.
func NewAnnounceService(cfg AnnounceServiceConfig, nodes []AnnounceNode) *AnnounceService {
	if cfg.PollInterval <= 0 {
		cfg.PollInterval = DefaultAnnouncePollInterval
	}
	if cfg.RepublishEvery <= 0 {
		cfg.RepublishEvery = DefaultAnnounceRepublishEvery
	}
	if cfg.PublishTimeout <= 0 {
		cfg.PublishTimeout = DefaultAnnouncePublishTimeout
	}
	if cfg.Now == nil {
		cfg.Now = time.Now
	}
	s := &AnnounceService{cfg: cfg, trigger: make(chan struct{}, 1)}
	for _, n := range nodes {
		s.nodes = append(s.nodes, &announceNodeState{AnnounceNode: n})
	}
	return s
}

// AddNode adds a node to publish for; safe while the service runs.
func (s *AnnounceService) AddNode(n AnnounceNode) {
	s.nodesMu.Lock()
	s.nodes = append(s.nodes, &announceNodeState{AnnounceNode: n})
	s.nodesMu.Unlock()
	s.Trigger()
}

func (s *AnnounceService) nodeList() []*announceNodeState {
	s.nodesMu.RLock()
	defer s.nodesMu.RUnlock()
	return append([]*announceNodeState(nil), s.nodes...)
}

// Start runs the publisher until ctx ends.
func (s *AnnounceService) Start(ctx context.Context) {
	ticker := time.NewTicker(s.cfg.PollInterval)
	defer ticker.Stop()
	for {
		s.RunOnce(ctx)
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		case <-s.trigger:
		}
	}
}

// Trigger asks for a run now (after a config update or a status change).
func (s *AnnounceService) Trigger() {
	select {
	case s.trigger <- struct{}{}:
	default:
	}
}

// RunOnce runs every node once.
func (s *AnnounceService) RunOnce(ctx context.Context) {
	s.runMu.Lock()
	defer s.runMu.Unlock()
	for _, n := range s.nodeList() {
		if err := s.runNode(ctx, n); err != nil {
			s.setError(n, err)
		}
	}
}

// Status returns the publisher status of the node, or nil when the
// publisher does not handle it.
func (s *AnnounceService) Status(nodeID []byte) *core.AnnouncePublisherStatus {
	n := s.node(nodeID)
	if n == nil {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	st := n.status
	return &st
}

// Recheck lifts a self-check block on the node and runs it again: the next
// publication is checked like any other.
func (s *AnnounceService) Recheck(nodeID []byte) bool {
	n := s.node(nodeID)
	if n == nil {
		return false
	}
	s.mu.Lock()
	n.status.Blocked = ""
	s.mu.Unlock()
	s.Trigger()
	return true
}

func (s *AnnounceService) node(nodeID []byte) *announceNodeState {
	for _, n := range s.nodeList() {
		if string(n.Node.NodeId) == string(nodeID) {
			return n
		}
	}
	return nil
}

func (s *AnnounceService) runNode(ctx context.Context, n *announceNodeState) error {
	cfg := n.Node.NodeConfig.Load()
	if !cfg.Announcing() {
		return nil
	}
	s.mu.Lock()
	blocked := n.status.Blocked != ""
	s.mu.Unlock()
	if blocked {
		return nil
	}

	state, err := s.cfg.Store.GetAnnounceState(ctx, n.Node.NodeId)
	if err != nil {
		return fmt.Errorf("reading the announcement status: %w", err)
	}
	if state.Status == core.AnnounceClosed && !state.ClosedPublishedAt.IsZero() {
		return nil // unlisted and announced as closed: nothing more
	}

	answers, err := n.Answers.LSPSAnswers()
	if err != nil {
		return fmt.Errorf("reading the LSPS answers: %w", err)
	}
	fields, err := s.buildFields(cfg, n, state.Status, answers)
	if err != nil {
		return err
	}
	fp, err := fingerprint(fields)
	if err != nil {
		return err
	}

	now := s.cfg.Now()
	due := n.fingerprint == "" || fp != n.fingerprint || now.Sub(n.lastAt) >= s.cfg.RepublishEvery ||
		state.Status == core.AnnounceClosed
	if !due {
		return nil
	}

	ev, err := niplsp.NewAnnouncement(fields, now.Unix())
	if err != nil {
		return fmt.Errorf("building the announcement: %w", err)
	}
	if err := niplsp.Sign(ctx, ev, n.Signer.SignPayload); err != nil {
		return fmt.Errorf("signing the announcement: %w", err)
	}

	results := s.publish(ctx, ev)
	pub := &core.AnnouncePublication{
		EventID:   ev.ID,
		CreatedAt: time.Unix(int64(ev.CreatedAt), 0),
		// Meta.Expiration is 0, so NewAnnouncement sets created_at + 30 days.
		Expiration: time.Unix(int64(ev.CreatedAt)+niplsp.MaxExpirySecs, 0),
		Status:     state.Status,
		Relays:     results,
	}
	accepted := 0
	for _, r := range results {
		if r.Accepted {
			accepted++
		}
	}
	s.mu.Lock()
	n.status.Last = pub
	s.mu.Unlock()
	if accepted == 0 {
		return fmt.Errorf("no relay accepted announcement %s", ev.ID)
	}

	n.fingerprint, n.lastAt = fp, now
	s.setError(n, nil)
	log.Info().Msgf("NIP-LSP: published announcement %s (status %s) to %d/%d relays", ev.ID, state.Status, accepted, len(results))

	if state.Status == core.AnnounceClosed {
		if _, err := s.cfg.Store.MarkClosedPublished(ctx, n.Node.NodeId, now); err != nil {
			return fmt.Errorf("recording the closed announcement: %w", err)
		}
	}

	s.selfCheck(n, cfg, ev, fp, now)
	return nil
}

// buildFields maps the live answers, the announce block and the status to
// niplsp Fields.
func (s *AnnounceService) buildFields(cfg *config.NodeConfig, n *announceNodeState, status core.AnnounceStatus, a *ports.LSPSAnswers) (niplsp.Fields, error) {
	ann := cfg.Announce
	display := niplsp.Display{
		Title:     ann.Title,
		Content:   ann.Content,
		Website:   ann.Website,
		Image:     ann.Image,
		Languages: ann.Languages,
	}
	if ann.Operator != nil && ann.Operator.Pubkey != "" {
		display.Operator = &niplsp.Operator{Pubkey: ann.Operator.Pubkey, Relay: ann.Operator.Relay}
	}
	fields, err := niplsp.FromLSPSInfo(a.Protocols, a.LSPS1Info, a.LSPS2Info, a.LSPS5, niplsp.Meta{
		NodeKey: hex.EncodeToString(n.Node.NodeId),
		Addrs:   ann.Addrs,
		Chain:   n.Chain,
		Network: n.Network,
		Status:  niplsp.Status(status),
		Display: display,
		Relays:  s.cfg.Relays,
	})
	if err != nil {
		return niplsp.Fields{}, fmt.Errorf("building the announcement fields: %w", err)
	}
	return fields, nil
}

// publish sends ev to every relay at once and collects their answers.
func (s *AnnounceService) publish(ctx context.Context, ev *nip01.Event) []core.AnnounceRelayResult {
	nev := toNostrEvent(ev)
	results := make([]core.AnnounceRelayResult, len(s.cfg.Relays))
	var wg sync.WaitGroup
	for i, url := range s.cfg.Relays {
		wg.Add(1)
		go func(i int, url string) {
			defer wg.Done()
			pctx, cancel := context.WithTimeout(ctx, s.cfg.PublishTimeout)
			defer cancel()
			r := core.AnnounceRelayResult{Relay: url, Accepted: true}
			if err := s.cfg.Publisher.Publish(pctx, url, nev); err != nil {
				r.Accepted, r.Error = false, err.Error()
				log.Warn().Err(err).Str("relay", url).Msg("NIP-LSP: relay did not accept the announcement")
			}
			results[i] = r
		}(i, url)
	}
	wg.Wait()
	return results
}

// selfCheck (T8-4) parses the published event as a client would and
// compares it with the live answers read again now. A difference whose
// cause is a value that changed since the build is not a fault: it asks for
// a republish. Any other failure blocks the node's publishing.
func (s *AnnounceService) selfCheck(n *announceNodeState, cfg *config.NodeConfig, ev *nip01.Event, publishedFP string, now time.Time) {
	check := &core.AnnounceSelfCheck{CheckedAt: now}
	block := func(problems ...string) {
		check.Problems = problems
		s.mu.Lock()
		n.status.SelfCheck = check
		n.status.Blocked = "self-check failed: " + problems[0]
		s.mu.Unlock()
		log.Error().Msgf("NIP-LSP: announcement %s failed its self-check, publishing stopped until rechecked: %v", ev.ID, problems)
	}

	a, err := niplsp.ParseAnnouncement(ev, niplsp.CheckOptions{
		Chains:   []string{n.Chain},
		Networks: []string{n.Network},
		Now:      now.Unix(),
	})
	if err != nil {
		block(fmt.Sprintf("the published event does not pass a client's checks: %v", err))
		return
	}

	fresh, err := n.Answers.LSPSAnswers()
	if err != nil {
		block(fmt.Sprintf("reading the live answers: %v", err))
		return
	}
	live := niplsp.Live{Protocols: fresh.Protocols}
	for _, p := range fresh.Protocols {
		switch p {
		case niplsp.ProtocolLSPS1:
			live.LSPS1, err = niplsp.ParseLSPS1Info(fresh.LSPS1Info)
		case niplsp.ProtocolLSPS2:
			live.LSPS2Menu, err = niplsp.ParseLSPS2Menu(fresh.LSPS2Info)
		}
		if err != nil {
			block(fmt.Sprintf("reading the live LSPS%d answer: %v", p, err))
			return
		}
	}

	diff := niplsp.CheckLive(a, live)
	if diff.Same() {
		check.OK = true
		s.mu.Lock()
		n.status.SelfCheck = check
		s.mu.Unlock()
		return
	}

	var problems []string
	for _, p := range diff.MissingProtocol {
		problems = append(problems, fmt.Sprintf("protocol %d is announced but not in list_protocols", p))
	}
	for _, f := range diff.Changed() {
		problems = append(problems, fmt.Sprintf("%s: announced %s, live %s (%s for the buyer)", f.Field, f.Announced, f.Live, f.Verdict))
	}

	// A live value that changed between the build and this check is a new
	// announced value, not a fault: republish it.
	if freshFields, err := s.buildFields(cfg, n, core.AnnounceStatus(a.Status), fresh); err == nil {
		if freshFP, err := fingerprint(freshFields); err == nil && freshFP != publishedFP {
			check.Problems = append(problems, "the live answers changed after publishing; republishing")
			s.mu.Lock()
			n.status.SelfCheck = check
			s.mu.Unlock()
			s.Trigger()
			return
		}
	}
	block(problems...)
}

func (s *AnnounceService) setError(n *announceNodeState, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	msg := ""
	if err != nil {
		msg = err.Error()
	}
	if msg != "" && msg != n.status.LastError {
		log.Error().Err(err).Msgf("NIP-LSP: announcing node %x", n.Node.NodeId)
	}
	n.status.LastError = msg
}

// fingerprint identifies the announced values; the expiration is left out
// (it is 0 in Meta until NewAnnouncement sets it), so only a real change of
// an announced value changes it.
func fingerprint(f niplsp.Fields) (string, error) {
	b, err := json.Marshal(f)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:]), nil
}

func toNostrEvent(ev *nip01.Event) nostr.Event {
	tags := make(nostr.Tags, len(ev.Tags))
	for i, t := range ev.Tags {
		tags[i] = nostr.Tag(append([]string(nil), t...))
	}
	return nostr.Event{
		ID:        ev.ID,
		PubKey:    ev.PubKey,
		CreatedAt: nostr.Timestamp(ev.CreatedAt),
		Kind:      ev.Kind,
		Tags:      tags,
		Content:   ev.Content,
		Sig:       ev.Sig,
	}
}
