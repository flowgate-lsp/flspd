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
// days when Meta.Expiration is 0). A relay that refuses is tried again after
// DefaultAnnounceBackoffBase, doubling up to DefaultAnnounceBackoffMax.
// A closed announcement is sent until every relay accepted it, or for
// DefaultAnnounceClosedRetryFor.
const (
	DefaultAnnouncePollInterval   = 30 * time.Second
	DefaultAnnounceRepublishEvery = 7 * 24 * time.Hour
	DefaultAnnouncePublishTimeout = 15 * time.Second
	DefaultAnnounceBackoffBase    = 30 * time.Second
	DefaultAnnounceBackoffMax     = 30 * time.Minute
	DefaultAnnounceClosedRetryFor = 24 * time.Hour
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
	BackoffBase    time.Duration
	BackoffMax     time.Duration
	ClosedRetryFor time.Duration
	Now            func() time.Time
}

// AnnounceService publishes each node's NIP-LSP announcement (T8-3) and
// checks every published event against the live answers (T8-4).
//
// On each run (at start, every PollInterval, and on Trigger) it works out
// the status to announce and builds the node's Fields from the live LSPS
// answers, the announce block and that status. It signs a new event when
// none is current, when an announced value differs from the current event
// (a hot-swap, a Pause/Resume/Unlist, a JIT switch), or when RepublishEvery
// has passed; then it sends the current event to every relay that has not
// accepted it and is not backing off.
//
// Closed is published once, then nothing more: after an Unlist (until a
// Resume), and when announcing is switched off on a node that published
// before (until it is switched on again; NIP-LSP: publish closed before it
// stops selling). "Once" means until every relay accepted it, or until
// ClosedRetryFor has passed with at least one acceptance: a relay that was
// down at the Unlist would otherwise keep the open announcement until it
// expires. A failed self-check stops publishing, across restarts,
// until Recheck; a closed withdrawal still goes out.
type AnnounceService struct {
	cfg     AnnounceServiceConfig
	trigger chan struct{}
	runMu   sync.Mutex // one run at a time
	mu      sync.Mutex // guards each node's status

	nodesMu sync.RWMutex
	nodes   []*announceNodeState
}

// closeReason is why a closed announcement goes out.
type closeReason int

const (
	notClosing closeReason = iota
	closingUnlist
	closingOff
)

type relayBackoff struct {
	failures int
	retryAt  time.Time
}

type announceNodeState struct {
	AnnounceNode

	// The current signed event and what it was built from.
	event       *nostr.Event
	fields      []byte // the event's Fields as JSON, stored on acceptance
	fingerprint string
	signedAt    time.Time
	eventStatus core.AnnounceStatus
	reason      closeReason
	accepted    map[string]bool
	results     map[string]core.AnnounceRelayResult
	recorded    bool // the first acceptance's bookkeeping is done
	// closingSince is when this node's current withdrawal was first signed.
	closingSince time.Time

	backoff map[string]*relayBackoff // per relay, reset on success

	// lastCreatedAt is the newest created_at signed in this process; the
	// stored one covers a restart.
	lastCreatedAt int64

	status core.AnnouncePublisherStatus
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
	if cfg.BackoffBase <= 0 {
		cfg.BackoffBase = DefaultAnnounceBackoffBase
	}
	if cfg.BackoffMax <= 0 {
		cfg.BackoffMax = DefaultAnnounceBackoffMax
	}
	if cfg.ClosedRetryFor <= 0 {
		cfg.ClosedRetryFor = DefaultAnnounceClosedRetryFor
	}
	if cfg.Now == nil {
		cfg.Now = time.Now
	}
	s := &AnnounceService{cfg: cfg, trigger: make(chan struct{}, 1)}
	for _, n := range nodes {
		s.nodes = append(s.nodes, newNodeState(n))
	}
	return s
}

func newNodeState(n AnnounceNode) *announceNodeState {
	return &announceNodeState{AnnounceNode: n, backoff: map[string]*relayBackoff{}}
}

// AddNode adds a node to publish for; safe while the service runs.
func (s *AnnounceService) AddNode(n AnnounceNode) {
	s.nodesMu.Lock()
	s.nodes = append(s.nodes, newNodeState(n))
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

// Recheck lifts a self-check block on the node (stored, so it holds across
// restarts until this) and runs it again: the next publication is checked
// like any other.
func (s *AnnounceService) Recheck(ctx context.Context, nodeID []byte) error {
	n := s.node(nodeID)
	if n == nil {
		return fmt.Errorf("the publisher does not handle this node")
	}
	if err := s.cfg.Store.SetSelfCheckBlock(ctx, nodeID, ""); err != nil {
		return fmt.Errorf("lifting the self-check block: %w", err)
	}
	s.mu.Lock()
	n.status.Blocked = ""
	s.mu.Unlock()
	s.Trigger()
	return nil
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
	state, err := s.cfg.Store.GetAnnounceState(ctx, n.Node.NodeId)
	if err != nil {
		return fmt.Errorf("reading the announcement status: %w", err)
	}
	s.mu.Lock()
	n.status.Blocked = state.SelfCheckBlock
	s.mu.Unlock()

	announcing := cfg.Announcing()
	if announcing && !state.OffClosedPublishedAt.IsZero() {
		// Switched on again: list the node again.
		if err := s.cfg.Store.ClearOffClosed(ctx, n.Node.NodeId); err != nil {
			return fmt.Errorf("clearing the switched-off mark: %w", err)
		}
		state.OffClosedPublishedAt = time.Time{}
	}

	status, reason := state.Status, notClosing
	switch {
	case !announcing:
		withdrawn := !state.OffClosedPublishedAt.IsZero() ||
			(state.Status == core.AnnounceClosed && !state.ClosedPublishedAt.IsZero())
		if state.LastPublishedAt.IsZero() || withdrawn {
			n.event = nil // nothing (more) to withdraw
			return nil
		}
		status, reason = core.AnnounceClosed, closingOff
	case state.Status == core.AnnounceClosed:
		if !state.ClosedPublishedAt.IsZero() {
			n.event = nil
			return nil
		}
		reason = closingUnlist
	default:
		if state.SelfCheckBlock != "" {
			return nil
		}
	}
	var fields niplsp.Fields
	if reason != notClosing {
		fields, err = s.closedFields(cfg, n, state)
	} else {
		fields, err = s.currentFields(cfg, n, status)
	}
	if err != nil {
		return err
	}
	fp, err := fingerprint(fields)
	if err != nil {
		return err
	}

	now := s.cfg.Now()
	if n.event == nil || fp != n.fingerprint || reason != n.reason || now.Sub(n.signedAt) >= s.cfg.RepublishEvery {
		createdAt := nextCreatedAt(now.Unix(), state.LastCreatedAt, n.lastCreatedAt)
		ev, err := niplsp.NewAnnouncement(fields, createdAt)
		if err != nil {
			return fmt.Errorf("building the announcement: %w", err)
		}
		if err := niplsp.Sign(ctx, ev, n.Signer.SignPayload); err != nil {
			return fmt.Errorf("signing the announcement: %w", err)
		}
		// Stored before any relay sees the event, so a restart signs after it.
		if err := s.cfg.Store.SetLastCreatedAt(ctx, n.Node.NodeId, createdAt); err != nil {
			return fmt.Errorf("recording the announcement's created_at: %w", err)
		}
		n.lastCreatedAt = createdAt
		fieldsJSON, err := json.Marshal(fields)
		if err != nil {
			return fmt.Errorf("encoding the announced fields: %w", err)
		}
		switch {
		case reason == notClosing:
			n.closingSince = time.Time{}
		case reason != n.reason || n.closingSince.IsZero():
			n.closingSince = now
		}
		nev := toNostrEvent(ev)
		n.event, n.fields, n.fingerprint, n.signedAt = &nev, fieldsJSON, fp, now
		n.eventStatus, n.reason = status, reason
		n.accepted, n.results, n.recorded = map[string]bool{}, map[string]core.AnnounceRelayResult{}, false
	}

	s.send(ctx, n, now)

	if len(n.accepted) == 0 {
		return fmt.Errorf("no relay accepted announcement %s yet", n.event.ID)
	}
	if !n.recorded {
		n.recorded = true
		s.setError(n, nil)
		log.Info().Msgf("NIP-LSP: published announcement %s (status %s) to %d/%d relays", n.event.ID, n.eventStatus, len(n.accepted), len(s.cfg.Relays))
		if err := s.cfg.Store.MarkPublished(ctx, n.Node.NodeId, now, n.fields); err != nil {
			return fmt.Errorf("recording the publication: %w", err)
		}
		if n.reason == notClosing {
			// A closed announcement is not shown by clients: nothing to compare.
			s.selfCheck(ctx, n, cfg, now)
		}
	}
	if n.reason == notClosing {
		return nil
	}

	// A withdrawal is done once every relay has it, or after ClosedRetryFor;
	// until then the same closed event is resent to the others.
	missing := s.missingRelays(n)
	if len(missing) > 0 {
		if now.Sub(n.closingSince) < s.cfg.ClosedRetryFor {
			return nil
		}
		log.Warn().Msgf("NIP-LSP: closed announcement %s never reached %v in %s; giving up on them", n.event.ID, missing, s.cfg.ClosedRetryFor)
	}
	switch n.reason {
	case closingUnlist:
		if _, err := s.cfg.Store.MarkClosedPublished(ctx, n.Node.NodeId, now); err != nil {
			return fmt.Errorf("recording the closed announcement: %w", err)
		}
	case closingOff:
		if err := s.cfg.Store.MarkOffClosedPublished(ctx, n.Node.NodeId, now); err != nil {
			return fmt.Errorf("recording the closed announcement: %w", err)
		}
	}
	return nil
}

// missingRelays are the relays that have not accepted the current event.
func (s *AnnounceService) missingRelays(n *announceNodeState) []string {
	var missing []string
	for _, url := range s.cfg.Relays {
		if !n.accepted[url] {
			missing = append(missing, url)
		}
	}
	return missing
}

// send sends the current event to every relay that has not accepted it and
// is not backing off, and records each answer. A refusal doubles the
// relay's backoff (BackoffBase up to BackoffMax); an acceptance resets it.
func (s *AnnounceService) send(ctx context.Context, n *announceNodeState, now time.Time) {
	type outcome struct {
		relay string
		err   error
	}
	var due []string
	for _, url := range s.cfg.Relays {
		if n.accepted[url] {
			continue
		}
		if b := n.backoff[url]; b != nil && now.Before(b.retryAt) {
			continue
		}
		due = append(due, url)
	}

	outcomes := make([]outcome, len(due))
	var wg sync.WaitGroup
	for i, url := range due {
		wg.Add(1)
		go func(i int, url string) {
			defer wg.Done()
			pctx, cancel := context.WithTimeout(ctx, s.cfg.PublishTimeout)
			defer cancel()
			outcomes[i] = outcome{url, s.cfg.Publisher.Publish(pctx, url, *n.event)}
		}(i, url)
	}
	wg.Wait()

	for _, o := range outcomes {
		if o.err == nil {
			n.accepted[o.relay] = true
			delete(n.backoff, o.relay)
			n.results[o.relay] = core.AnnounceRelayResult{Relay: o.relay, Accepted: true}
			continue
		}
		b := n.backoff[o.relay]
		if b == nil {
			b = &relayBackoff{}
			n.backoff[o.relay] = b
		}
		b.failures++
		b.retryAt = now.Add(backoffDelay(b.failures, s.cfg.BackoffBase, s.cfg.BackoffMax))
		n.results[o.relay] = core.AnnounceRelayResult{Relay: o.relay, Error: o.err.Error(), RetryAt: b.retryAt}
		log.Warn().Err(o.err).Str("relay", o.relay).Msgf("NIP-LSP: relay did not accept the announcement; next try at %s", b.retryAt.UTC().Format(time.RFC3339))
	}

	// A fresh copy for Status readers.
	pub := &core.AnnouncePublication{
		EventID:    n.event.ID,
		CreatedAt:  n.event.CreatedAt.Time(),
		Expiration: n.event.CreatedAt.Time().Add(niplsp.MaxExpirySecs * time.Second),
		Status:     n.eventStatus,
	}
	for _, url := range s.cfg.Relays {
		r, ok := n.results[url]
		if !ok {
			r = core.AnnounceRelayResult{Relay: url, Error: "not tried yet: backing off"}
			if b := n.backoff[url]; b != nil {
				r.RetryAt = b.retryAt
			}
		}
		pub.Relays = append(pub.Relays, r)
	}
	s.mu.Lock()
	n.status.Last = pub
	s.mu.Unlock()
}

// nextCreatedAt is now, or one second after the newest created_at already
// signed when that is not before now. Relays keep the newest event per node
// and, on a created_at tie, the one with the lowest id (NIP-01), so a status
// change signed in the same second as the event before it, or after the
// clock went back, must still be newer.
func nextCreatedAt(now int64, last ...int64) int64 {
	for _, l := range last {
		if now <= l {
			now = l + 1
		}
	}
	return now
}

// backoffDelay is base doubled per failure after the first, capped at max.
func backoffDelay(failures int, base, max time.Duration) time.Duration {
	d := base
	for i := 1; i < failures && d < max; i++ {
		d *= 2
	}
	if d > max {
		d = max
	}
	return d
}

// currentFields builds the node's Fields from its config and live answers.
func (s *AnnounceService) currentFields(cfg *config.NodeConfig, n *announceNodeState, status core.AnnounceStatus) (niplsp.Fields, error) {
	if cfg.Announce == nil {
		return niplsp.Fields{}, fmt.Errorf("the announce block is gone, so status %s cannot be announced", status)
	}
	answers, err := n.Answers.LSPSAnswers()
	if err != nil {
		return niplsp.Fields{}, fmt.Errorf("reading the LSPS answers: %w", err)
	}
	return s.buildFields(cfg, n, status, answers)
}

// closedFields are the withdrawal's Fields: the last announced values with
// status closed. They do not depend on the config or the live answers,
// which may no longer build an announcement once announcing is switched off
// (the announce block removed, addrs cleared) or the LSPS answers changed.
// A node with no stored values (published before they were kept) is built
// from the current config.
func (s *AnnounceService) closedFields(cfg *config.NodeConfig, n *announceNodeState, state *core.AnnounceState) (niplsp.Fields, error) {
	if len(state.LastFields) > 0 {
		var f niplsp.Fields
		err := json.Unmarshal(state.LastFields, &f)
		if err == nil {
			f.Status = niplsp.StatusClosed
			f.Expiration = 0
			f.Relays = s.cfg.Relays
			return f, nil
		}
		log.Warn().Err(err).Msg("NIP-LSP: the stored announcement values do not decode; building closed from the config")
	}
	return s.currentFields(cfg, n, core.AnnounceClosed)
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

// selfCheck (T8-4) parses the published event as a client would and
// compares it with the live answers read again now. A difference whose
// cause is a value that changed since the build is not a fault: it asks for
// a republish. Any other failure blocks the node's publishing.
func (s *AnnounceService) selfCheck(ctx context.Context, n *announceNodeState, cfg *config.NodeConfig, now time.Time) {
	ev, publishedFP := fromNostrEvent(*n.event), n.fingerprint
	check := &core.AnnounceSelfCheck{CheckedAt: now}
	block := func(problems ...string) {
		check.Problems = problems
		reason := "self-check failed: " + problems[0]
		s.mu.Lock()
		n.status.SelfCheck = check
		n.status.Blocked = reason
		s.mu.Unlock()
		// Stored, so a restart does not lift it; only Recheck does.
		if err := s.cfg.Store.SetSelfCheckBlock(ctx, n.Node.NodeId, reason); err != nil {
			log.Error().Err(err).Msg("NIP-LSP: storing the self-check block")
		}
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

func fromNostrEvent(ev nostr.Event) *nip01.Event {
	tags := make([][]string, len(ev.Tags))
	for i, t := range ev.Tags {
		tags[i] = append([]string(nil), t...)
	}
	return &nip01.Event{
		ID:        ev.ID,
		PubKey:    ev.PubKey,
		CreatedAt: uint64(ev.CreatedAt),
		Kind:      ev.Kind,
		Tags:      tags,
		Content:   ev.Content,
		Sig:       ev.Sig,
	}
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
