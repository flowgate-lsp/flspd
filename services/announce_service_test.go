package services

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/flowgate-lsp/flspd/core"
	"github.com/flowgate-lsp/flspd/platform/config"
	"github.com/flowgate-lsp/flspd/ports"
	"github.com/nbd-wtf/go-nostr"
	"github.com/ohstr/nmilat/niplsp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// LSPS answers as flspd's handlers give them (the _loki/_mloki dialect).
const (
	testLSPS1Info = `{"options":["lsps1.create_order","lsps1.get_order"],
		"min_required_channel_confirmations":0,"min_funding_confirms_within_blocks":1,
		"supports_zero_channel_reserve":true,"max_channel_expiry_blocks":201600,
		"min_initial_client_balance_loki":"0","max_initial_client_balance_loki":"100000000",
		"min_initial_lsp_balance_loki":"0","max_initial_lsp_balance_loki":"100000000",
		"min_channel_balance_loki":"20000","max_channel_balance_loki":"100000000"}`
	testLSPS2Info = `{"opening_fee_params_menu":[{"min_fee_mloki":"1000","proportional":4000,
		"valid_until":"2026-10-27T12:00:00.000Z","min_lifetime":43200,"max_client_to_self_delay":10080,
		"min_payment_size_mloki":"20000000","max_payment_size_mloki":"10000000000","promise":"abc"}]}`
)

func testAnswers(protocols ...int) *ports.LSPSAnswers {
	a := &ports.LSPSAnswers{Protocols: protocols}
	for _, p := range protocols {
		switch p {
		case 1:
			a.LSPS1Info = json.RawMessage(testLSPS1Info)
		case 2:
			a.LSPS2Info = json.RawMessage(testLSPS2Info)
		case 5:
			a.LSPS5 = json.RawMessage(`{"max_webhooks":10}`)
		}
	}
	return a
}

// fakeAnswers returns next() on each call.
type fakeAnswers struct {
	mu   sync.Mutex
	next func(call int) (*ports.LSPSAnswers, error)
	n    int
}

func (f *fakeAnswers) LSPSAnswers() (*ports.LSPSAnswers, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.n++
	return f.next(f.n)
}

func constAnswers(a *ports.LSPSAnswers) *fakeAnswers {
	return &fakeAnswers{next: func(int) (*ports.LSPSAnswers, error) { return a, nil }}
}

// memAnnounceStore is an in-memory ports.AnnounceStateStore with the real
// transition rules.
type memAnnounceStore struct {
	mu     sync.Mutex
	states map[string]core.AnnounceState
}

func newMemAnnounceStore() *memAnnounceStore {
	return &memAnnounceStore{states: map[string]core.AnnounceState{}}
}

func (m *memAnnounceStore) GetAnnounceState(_ context.Context, id []byte) (*core.AnnounceState, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	st, ok := m.states[string(id)]
	if !ok {
		st = core.AnnounceState{NodeID: id, Status: core.AnnounceOpen}
	}
	return &st, nil
}

func (m *memAnnounceStore) ApplyAnnounceAction(ctx context.Context, id []byte, a core.AnnounceAction) (*core.AnnounceState, error) {
	cur, _ := m.GetAnnounceState(ctx, id)
	next, err := core.NextAnnounceStatus(cur.Status, a)
	if err != nil {
		return nil, err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if next != cur.Status {
		cur.Status, cur.UpdatedAt, cur.ClosedPublishedAt = next, time.Now(), time.Time{}
	}
	m.states[string(id)] = *cur
	return cur, nil
}

func (m *memAnnounceStore) MarkClosedPublished(_ context.Context, id []byte, at time.Time) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	st, ok := m.states[string(id)]
	if !ok || st.Status != core.AnnounceClosed || !st.ClosedPublishedAt.IsZero() {
		return false, nil
	}
	st.ClosedPublishedAt = at
	m.states[string(id)] = st
	return true, nil
}

func (m *memAnnounceStore) update(id []byte, f func(st *core.AnnounceState)) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	st, ok := m.states[string(id)]
	if !ok {
		st = core.AnnounceState{NodeID: id, Status: core.AnnounceOpen}
	}
	f(&st)
	m.states[string(id)] = st
	return nil
}

func (m *memAnnounceStore) MarkPublished(_ context.Context, id []byte, at time.Time, fields []byte) error {
	return m.update(id, func(st *core.AnnounceState) {
		st.LastPublishedAt, st.LastFields = at, append([]byte(nil), fields...)
	})
}

func (m *memAnnounceStore) SetLastCreatedAt(_ context.Context, id []byte, createdAt int64) error {
	return m.update(id, func(st *core.AnnounceState) { st.LastCreatedAt = createdAt })
}

func (m *memAnnounceStore) MarkOffClosedPublished(_ context.Context, id []byte, at time.Time) error {
	return m.update(id, func(st *core.AnnounceState) { st.OffClosedPublishedAt = at })
}

func (m *memAnnounceStore) ClearOffClosed(_ context.Context, id []byte) error {
	return m.update(id, func(st *core.AnnounceState) { st.OffClosedPublishedAt = time.Time{} })
}

func (m *memAnnounceStore) SetSelfCheckBlock(_ context.Context, id []byte, reason string) error {
	return m.update(id, func(st *core.AnnounceState) { st.SelfCheckBlock = reason })
}

// fakeRelays records what each relay was sent; reject makes a relay refuse.
type fakeRelays struct {
	mu       sync.Mutex
	events   []nostr.Event
	reject   map[string]bool
	attempts map[string]int
}

func (f *fakeRelays) tries(url string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.attempts[url]
}

func (f *fakeRelays) setReject(r map[string]bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.reject = r
}

func (f *fakeRelays) Publish(_ context.Context, url string, ev nostr.Event) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.attempts == nil {
		f.attempts = map[string]int{}
	}
	f.attempts[url]++
	if f.reject[url] {
		return errors.New("blocked: no")
	}
	f.events = append(f.events, ev)
	return nil
}

func (f *fakeRelays) count() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.events)
}

func (f *fakeRelays) last() nostr.Event {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.events[len(f.events)-1]
}

type announceFixture struct {
	svc    *AnnounceService
	node   *core.Node
	store  *memAnnounceStore
	relays *fakeRelays
	now    *time.Time
}

var testRelays = []string{"wss://a.example", "wss://b.example"}

func newAnnounceFixture(t *testing.T, answers ports.LSPSAnswerSource) *announceFixture {
	t.Helper()
	priv := keyWithPrefix(t, 0x03)
	cfg := &config.NodeConfig{
		Name: "lsp",
		Announce: &config.AnnounceConfig{
			Enabled:   true,
			Title:     "Test LSP",
			Content:   "JIT channels.",
			Website:   "https://lsp.example.com",
			Operator:  &config.AnnounceOperator{Pubkey: "c6047f9441ed7d6d3045406e95c07cd85c778e4b8cef3ca7abac09b95c709ee5", Relay: "wss://op.example"},
			Languages: []string{"en"},
			Addrs:     []string{"203.0.113.7:9735"},
		},
	}
	node := newTestNode(t, &fakeNodeKeySigner{priv: priv}, priv)
	node.NodeConfig = config.NewNodeConfigRef(cfg)
	signer, err := NewAnnouncementSigner(node)
	require.NoError(t, err)

	now := time.Unix(1792713600, 0)
	f := &announceFixture{node: node, store: newMemAnnounceStore(), relays: &fakeRelays{reject: map[string]bool{}}, now: &now}
	f.svc = NewAnnounceService(AnnounceServiceConfig{
		Relays:    testRelays,
		Store:     f.store,
		Publisher: f.relays,
		Now:       func() time.Time { return *f.now },
	}, []AnnounceNode{{Node: node, Signer: signer, Answers: answers, Chain: "flokicoin", Network: "regtest"}})
	return f
}

func (f *announceFixture) run() { f.svc.RunOnce(context.Background()) }

func (f *announceFixture) status() *core.AnnouncePublisherStatus { return f.svc.Status(f.node.NodeId) }

func (f *announceFixture) setConfig(mut func(c *config.NodeConfig)) {
	c := *f.node.NodeConfig.Load()
	a := *c.Announce
	c.Announce = &a
	mut(&c)
	f.node.NodeConfig.Store(&c)
}

func tagValue(ev nostr.Event, name string) string {
	for _, t := range ev.Tags {
		if len(t) >= 2 && t[0] == name {
			return t[1]
		}
	}
	return ""
}

func TestAnnounceService_PublishesSignedCheckedAnnouncement(t *testing.T) {
	f := newAnnounceFixture(t, constAnswers(testAnswers(1, 2, 5)))
	f.run()

	require.Equal(t, 2, f.relays.count(), "sent to both relays")
	ev := f.relays.last()
	ok, err := ev.CheckSignature()
	require.NoError(t, err)
	assert.True(t, ok, "signed by the node key")
	assert.Equal(t, niplsp.Kind, ev.Kind)
	assert.Equal(t, ev.GetID(), ev.ID)
	assert.Equal(t, "203.0.113.7:9735", tagValue(ev, "addr"))
	assert.Equal(t, "flokicoin", tagValue(ev, "n"))
	assert.Equal(t, "regtest", tagValue(ev, "network"))
	assert.Equal(t, "open", tagValue(ev, "status"))
	assert.Equal(t, "Test LSP", tagValue(ev, "title"))
	assert.Equal(t, "1000", tagValue(ev, "lsps2_fee"), "min_fee in millis, valid_until and promise dropped")
	assert.Equal(t, "JIT channels.", ev.Content)

	var protocols, relays []string
	for _, tg := range ev.Tags {
		switch tg[0] {
		case "protocol":
			protocols = append(protocols, tg[1])
		case "r":
			relays = append(relays, tg[1])
		}
	}
	assert.Equal(t, []string{"1", "2", "5"}, protocols)
	assert.Equal(t, testRelays, relays)
	assert.Equal(t, "1795305600", tagValue(ev, "expiration"), "created_at + 30 days")

	st := f.status()
	require.NotNil(t, st.Last)
	assert.Equal(t, ev.ID, st.Last.EventID)
	assert.Equal(t, core.AnnounceOpen, st.Last.Status)
	assert.Equal(t, int64(1795305600), st.Last.Expiration.Unix())
	require.Len(t, st.Last.Relays, 2)
	assert.True(t, st.Last.Relays[0].Accepted && st.Last.Relays[1].Accepted)
	require.NotNil(t, st.SelfCheck)
	assert.True(t, st.SelfCheck.OK, "self-check problems: %v", st.SelfCheck.Problems)
	assert.Empty(t, st.Blocked)
	assert.Empty(t, st.LastError)
}

func TestAnnounceService_RepublishRules(t *testing.T) {
	answers := testAnswers(1, 2, 5)
	src := &fakeAnswers{next: func(int) (*ports.LSPSAnswers, error) { return answers, nil }}
	f := newAnnounceFixture(t, src)
	f.run()
	require.Equal(t, 2, f.relays.count())

	// Nothing changed: nothing published.
	*f.now = f.now.Add(time.Hour)
	f.run()
	assert.Equal(t, 2, f.relays.count())

	// A hot-swap of an announced value republishes.
	f.setConfig(func(c *config.NodeConfig) { c.Announce.Title = "Renamed LSP" })
	f.run()
	require.Equal(t, 4, f.relays.count())
	assert.Equal(t, "Renamed LSP", tagValue(f.relays.last(), "title"))

	// A changed live answer (JIT switched off) republishes without protocol 2.
	answers = testAnswers(1, 5)
	f.run()
	require.Equal(t, 6, f.relays.count())
	assert.Empty(t, tagValue(f.relays.last(), "lsps2_fee"))

	// A value outside the announcement (the node name) changes nothing.
	f.setConfig(func(c *config.NodeConfig) { c.Name = "other" })
	f.run()
	assert.Equal(t, 6, f.relays.count())

	// After 7 days the same values are republished with a new expiration.
	*f.now = f.now.Add(7 * 24 * time.Hour)
	f.run()
	require.Equal(t, 8, f.relays.count())
	assert.Equal(t, f.now.Unix()+niplsp.MaxExpirySecs, f.status().Last.Expiration.Unix())
}

func TestAnnounceService_PauseResumeUnlist(t *testing.T) {
	f := newAnnounceFixture(t, constAnswers(testAnswers(1, 2, 5)))
	ctx := context.Background()
	f.run()
	require.Equal(t, "open", tagValue(f.relays.last(), "status"))

	_, err := f.store.ApplyAnnounceAction(ctx, f.node.NodeId, core.AnnouncePause)
	require.NoError(t, err)
	f.run()
	assert.Equal(t, "paused", tagValue(f.relays.last(), "status"))

	_, err = f.store.ApplyAnnounceAction(ctx, f.node.NodeId, core.AnnounceUnlist)
	require.NoError(t, err)
	f.run()
	assert.Equal(t, "closed", tagValue(f.relays.last(), "status"))
	sent := f.relays.count()
	st, _ := f.store.GetAnnounceState(ctx, f.node.NodeId)
	assert.False(t, st.ClosedPublishedAt.IsZero(), "closed recorded as published")

	// Closed is published once, then nothing, even after 7 days or a change.
	*f.now = f.now.Add(8 * 24 * time.Hour)
	f.setConfig(func(c *config.NodeConfig) { c.Announce.Title = "Gone" })
	f.run()
	assert.Equal(t, sent, f.relays.count())

	// A publisher restarted after the Unlist does not publish it again.
	f2 := &announceFixture{node: f.node, store: f.store, relays: f.relays, now: f.now}
	f2.svc = NewAnnounceService(AnnounceServiceConfig{Relays: testRelays, Store: f.store, Publisher: f.relays, Now: func() time.Time { return *f.now }},
		[]AnnounceNode{f.svc.nodeList()[0].AnnounceNode})
	f2.run()
	assert.Equal(t, sent, f.relays.count())

	// Resume lists the node again.
	_, err = f.store.ApplyAnnounceAction(ctx, f.node.NodeId, core.AnnounceResume)
	require.NoError(t, err)
	f.run()
	assert.Equal(t, "open", tagValue(f.relays.last(), "status"))
}

// F-M1: a closed announcement is resent until every relay accepted it, so
// a relay that was down at the Unlist does not keep the open one.
func TestAnnounceService_ClosedRetriedUntilEveryRelayAccepts(t *testing.T) {
	ctx := context.Background()
	f := newAnnounceFixture(t, constAnswers(testAnswers(1)))
	f.run()
	require.Equal(t, "open", tagValue(f.relays.last(), "status"))

	_, _ = f.store.ApplyAnnounceAction(ctx, f.node.NodeId, core.AnnounceUnlist)
	f.relays.setReject(map[string]bool{testRelays[0]: true, testRelays[1]: true})
	f.run()
	st, _ := f.store.GetAnnounceState(ctx, f.node.NodeId)
	assert.True(t, st.ClosedPublishedAt.IsZero(), "not recorded while no relay accepted")
	assert.Contains(t, f.status().LastError, "no relay accepted")

	// One relay accepts: still not done, the other is retried.
	f.relays.setReject(map[string]bool{testRelays[0]: true})
	*f.now = f.now.Add(time.Minute) // past the first backoff
	f.run()
	st, _ = f.store.GetAnnounceState(ctx, f.node.NodeId)
	assert.True(t, st.ClosedPublishedAt.IsZero(), "a relay still serves the open announcement")
	assert.Empty(t, f.status().LastError)
	rel := f.status().Last.Relays
	assert.False(t, rel[0].Accepted)
	assert.Contains(t, rel[0].Error, "blocked")
	assert.False(t, rel[0].RetryAt.IsZero())
	assert.True(t, rel[1].Accepted)
	closedID := f.status().Last.EventID

	// A restart in between keeps withdrawing.
	f.svc = NewAnnounceService(AnnounceServiceConfig{Relays: testRelays, Store: f.store, Publisher: f.relays, Now: func() time.Time { return *f.now }},
		[]AnnounceNode{f.svc.nodeList()[0].AnnounceNode})

	// The relay comes back: it gets closed, and only then is it recorded.
	f.relays.setReject(map[string]bool{})
	*f.now = f.now.Add(time.Hour)
	before := f.relays.tries(testRelays[0])
	f.run()
	assert.Equal(t, before+1, f.relays.tries(testRelays[0]))
	assert.Equal(t, "closed", tagValue(f.relays.last(), "status"))
	assert.NotEqual(t, closedID, f.relays.last().ID, "re-signed after the restart")
	st, _ = f.store.GetAnnounceState(ctx, f.node.NodeId)
	assert.False(t, st.ClosedPublishedAt.IsZero())

	// Done: nothing more is sent.
	sent := f.relays.count()
	*f.now = f.now.Add(time.Hour)
	f.run()
	assert.Equal(t, sent, f.relays.count())
}

// F-M1: a relay that never accepts the withdrawal is given up on after
// ClosedRetryFor, so switching off does not retry forever.
func TestAnnounceService_ClosedGivesUpOnARelayAfterRetryFor(t *testing.T) {
	ctx := context.Background()
	f := newAnnounceFixture(t, constAnswers(testAnswers(1)))
	f.run()
	f.relays.setReject(map[string]bool{testRelays[0]: true})
	f.setConfig(func(c *config.NodeConfig) { c.Announce.Enabled = false })
	start := *f.now
	for d := time.Duration(0); d < DefaultAnnounceClosedRetryFor; d += 30 * time.Minute {
		*f.now = start.Add(d)
		f.run()
		st, _ := f.store.GetAnnounceState(ctx, f.node.NodeId)
		require.True(t, st.OffClosedPublishedAt.IsZero(), "still retrying at %s", d)
	}
	*f.now = start.Add(DefaultAnnounceClosedRetryFor)
	f.run()
	st, _ := f.store.GetAnnounceState(ctx, f.node.NodeId)
	assert.False(t, st.OffClosedPublishedAt.IsZero(), "given up after %s", DefaultAnnounceClosedRetryFor)
	tries := f.relays.tries(testRelays[0])
	*f.now = f.now.Add(time.Hour)
	f.run()
	assert.Equal(t, tries, f.relays.tries(testRelays[0]))
}

func TestAnnounceService_FailedPublishIsRetried(t *testing.T) {
	f := newAnnounceFixture(t, constAnswers(testAnswers(1, 5)))
	f.relays.reject = map[string]bool{testRelays[0]: true, testRelays[1]: true}
	f.run()
	assert.Equal(t, 0, f.relays.count())
	assert.NotEmpty(t, f.status().LastError)

	firstID := f.status().Last.EventID
	f.relays.setReject(map[string]bool{})
	f.run()
	assert.Equal(t, 0, f.relays.count(), "backing off: not tried again at once")
	*f.now = f.now.Add(time.Minute)
	f.run()
	assert.Equal(t, 2, f.relays.count(), "the unchanged values are published once a relay accepts")
	assert.Equal(t, firstID, f.relays.last().ID, "the same signed event is resent")
}

func TestAnnounceService_NotAnnouncing(t *testing.T) {
	f := newAnnounceFixture(t, constAnswers(testAnswers(1, 2, 5)))
	f.setConfig(func(c *config.NodeConfig) { c.Announce.Enabled = false })
	f.run()
	assert.Equal(t, 0, f.relays.count())
	assert.Nil(t, f.status().Last)
}

func TestAnnounceService_BuildErrorsAreReported(t *testing.T) {
	// Protocol 2 listed without a menu: FromLSPSInfo refuses.
	f := newAnnounceFixture(t, constAnswers(&ports.LSPSAnswers{Protocols: []int{2}}))
	f.run()
	assert.Equal(t, 0, f.relays.count())
	assert.Contains(t, f.status().LastError, "announcement fields")
}

// armedAnswers gives good answers, except that the first read after a
// publish (the self-check's) gets bad() while broken is set.
type armedAnswers struct {
	mu     sync.Mutex
	armed  bool
	broken bool
	bad    func() (*ports.LSPSAnswers, error)
}

func (a *armedAnswers) LSPSAnswers() (*ports.LSPSAnswers, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.armed && a.broken {
		a.armed = false
		return a.bad()
	}
	return testAnswers(1, 2, 5), nil
}

func (a *armedAnswers) set(broken bool) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.broken = broken
}

func newArmedFixture(t *testing.T, bad func() (*ports.LSPSAnswers, error)) (*announceFixture, *armedAnswers) {
	src := &armedAnswers{bad: bad}
	f := newAnnounceFixture(t, src)
	pub := f.svc.cfg.Publisher
	f.svc.cfg.Publisher = publishFunc(func(ctx context.Context, url string, ev nostr.Event) error {
		src.mu.Lock()
		src.armed = true
		src.mu.Unlock()
		return pub.Publish(ctx, url, ev)
	})
	return f, src
}

func unreadableLSPS2() (*ports.LSPSAnswers, error) {
	a := testAnswers(1, 2, 5)
	a.LSPS2Info = json.RawMessage(`{"opening_fee_params_menu":"oops"}`)
	return a, nil
}

// T8-4, F-M3: a published announcement that does not match the live
// answers, while those answers did not change, blocks the node until
// Recheck. While blocked it is announced as paused and still follows every
// change, rather than leaving the faulty event live.
func TestAnnounceService_SelfCheckBlockAnnouncesPausedUntilRecheck(t *testing.T) {
	ctx := context.Background()
	f, src := newArmedFixture(t, unreadableLSPS2)
	src.set(true)
	f.run()
	require.Equal(t, 2, f.relays.count())
	assert.Equal(t, "open", tagValue(f.relays.last(), "status"))
	st := f.status()
	require.NotNil(t, st.SelfCheck)
	assert.False(t, st.SelfCheck.OK)
	assert.Contains(t, st.Blocked, "self-check failed")
	select {
	case <-f.svc.trigger:
	default:
		t.Fatal("no run requested to publish paused")
	}
	// The answers are good again; the stored block alone keeps it paused.
	src.set(false)

	// The next run replaces the faulty event with paused.
	f.run()
	require.Equal(t, 4, f.relays.count())
	assert.Equal(t, "paused", tagValue(f.relays.last(), "status"))
	stored, _ := f.store.GetAnnounceState(ctx, f.node.NodeId)
	assert.Equal(t, core.AnnounceOpen, stored.Status, "the stored status is untouched")

	// Blocked: a change still goes out, as paused.
	f.setConfig(func(c *config.NodeConfig) { c.Announce.Title = "Renamed" })
	f.run()
	require.Equal(t, 6, f.relays.count())
	assert.Equal(t, "paused", tagValue(f.relays.last(), "status"))
	assert.Equal(t, "Renamed", tagValue(f.relays.last(), "title"))

	// The block is stored: a restarted publisher still announces paused.
	stored, _ = f.store.GetAnnounceState(ctx, f.node.NodeId)
	assert.Contains(t, stored.SelfCheckBlock, "self-check failed")
	restarted := NewAnnounceService(AnnounceServiceConfig{Relays: testRelays, Store: f.store, Publisher: f.relays, Now: func() time.Time { return *f.now }},
		[]AnnounceNode{f.svc.nodeList()[0].AnnounceNode})
	restarted.RunOnce(ctx)
	assert.Equal(t, "paused", tagValue(f.relays.last(), "status"), "still paused after a restart")
	assert.Contains(t, restarted.Status(f.node.NodeId).Blocked, "self-check failed")

	// Rechecked: the stored status is announced and checked again.
	require.NoError(t, f.svc.Recheck(ctx, f.node.NodeId))
	stored, _ = f.store.GetAnnounceState(ctx, f.node.NodeId)
	assert.Empty(t, stored.SelfCheckBlock)
	sent := f.relays.count()
	f.run()
	assert.Equal(t, sent+2, f.relays.count())
	assert.Equal(t, "open", tagValue(f.relays.last(), "status"))
	st = f.status()
	assert.Empty(t, st.Blocked)
	assert.True(t, st.SelfCheck.OK, "%v", st.SelfCheck.Problems)
	assert.Error(t, f.svc.Recheck(ctx, []byte{1}), "unknown node")
}

// F-M3: a Pause while blocked is announced; it was withheld before.
func TestAnnounceService_PauseWhileBlocked(t *testing.T) {
	ctx := context.Background()
	f, src := newArmedFixture(t, unreadableLSPS2)
	src.set(true)
	f.run()
	src.set(false)
	f.run()
	require.Equal(t, "paused", tagValue(f.relays.last(), "status"))
	_, err := f.store.ApplyAnnounceAction(ctx, f.node.NodeId, core.AnnouncePause)
	require.NoError(t, err)
	f.run()
	assert.Equal(t, "paused", tagValue(f.relays.last(), "status"))

	// Rechecked while the operator's Pause stands: still paused, and checked.
	require.NoError(t, f.svc.Recheck(ctx, f.node.NodeId))
	sent := f.relays.count()
	f.run()
	assert.Equal(t, sent+2, f.relays.count(), "republished unblocked, to be checked")
	assert.Equal(t, "paused", tagValue(f.relays.last(), "status"))
	assert.True(t, f.status().SelfCheck.OK, "%v", f.status().SelfCheck.Problems)
}

// F-M3: a self-check that cannot read the live answers is retried on the
// next run, not stored as a block.
func TestAnnounceService_SelfCheckReadErrorIsRetried(t *testing.T) {
	f, src := newArmedFixture(t, func() (*ports.LSPSAnswers, error) { return nil, errors.New("db down") })
	src.set(true)
	f.run()
	st := f.status()
	assert.Empty(t, st.Blocked)
	require.NotNil(t, st.SelfCheck)
	assert.False(t, st.SelfCheck.OK)
	assert.Contains(t, st.SelfCheck.Problems[0], "checking again")
	stored, _ := f.store.GetAnnounceState(context.Background(), f.node.NodeId)
	assert.Empty(t, stored.SelfCheckBlock)

	src.set(false)
	f.run()
	assert.Equal(t, 2, f.relays.count(), "the same event, not republished")
	assert.True(t, f.status().SelfCheck.OK, "%v", f.status().SelfCheck.Problems)
}

// T8-4: a live value that changed between publishing and the check is a
// new value to announce, not a fault.
func TestAnnounceService_SelfCheckRepublishesChangedValues(t *testing.T) {
	var cheaper atomic.Bool
	src := &fakeAnswers{next: func(int) (*ports.LSPSAnswers, error) {
		a := testAnswers(1, 2, 5)
		if cheaper.Load() {
			a.LSPS2Info = json.RawMessage(strings.Replace(testLSPS2Info, `"min_fee_mloki":"1000"`, `"min_fee_mloki":"500"`, 1))
		}
		return a, nil
	}}
	f := newAnnounceFixture(t, src)
	// The value changes right after the build: flip on the first publish.
	f.relays.reject = map[string]bool{}
	pub := f.svc.cfg.Publisher
	f.svc.cfg.Publisher = publishFunc(func(ctx context.Context, url string, ev nostr.Event) error {
		cheaper.Store(true)
		return pub.Publish(ctx, url, ev)
	})
	f.run()
	st := f.status()
	require.NotNil(t, st.SelfCheck)
	assert.False(t, st.SelfCheck.OK)
	assert.Empty(t, st.Blocked, "not a fault")
	assert.Contains(t, strings.Join(st.SelfCheck.Problems, "; "), "republishing")

	select {
	case <-f.svc.trigger:
	default:
		t.Fatal("no republish requested")
	}
	f.run()
	assert.Equal(t, "500", tagValue(f.relays.last(), "lsps2_fee"))
	assert.True(t, f.status().SelfCheck.OK)
}

type publishFunc func(ctx context.Context, url string, ev nostr.Event) error

func (p publishFunc) Publish(ctx context.Context, url string, ev nostr.Event) error {
	return p(ctx, url, ev)
}

func TestAnnounceService_TriggerDoesNotBlock(t *testing.T) {
	f := newAnnounceFixture(t, constAnswers(testAnswers(1)))
	for i := 0; i < 5; i++ {
		f.svc.Trigger()
	}
	assert.Nil(t, f.svc.Status([]byte{1}))
}

func TestAnnounceService_StartRunsAndStops(t *testing.T) {
	f := newAnnounceFixture(t, constAnswers(testAnswers(1)))
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { f.svc.Start(ctx); close(done) }()
	require.Eventually(t, func() bool { return f.relays.count() == 2 }, 5*time.Second, 10*time.Millisecond)
	cancel()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Start did not return")
	}
}

func TestAnnounceService_AddNodeWhileRunning(t *testing.T) {
	f := newAnnounceFixture(t, constAnswers(testAnswers(1)))
	n := f.svc.nodeList()[0].AnnounceNode
	svc := NewAnnounceService(AnnounceServiceConfig{Relays: testRelays, Store: f.store, Publisher: f.relays}, nil)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go svc.Start(ctx)
	assert.Nil(t, svc.Status(n.Node.NodeId))
	svc.AddNode(n)
	require.Eventually(t, func() bool { return f.relays.count() == 2 }, 5*time.Second, 10*time.Millisecond)
	require.NotNil(t, svc.Status(n.Node.NodeId))
}

// Switching announcing off withdraws the node like an Unlist: closed is
// published once (NIP-LSP: publish closed before it stops selling), and
// switching it on again lists the node again.
func TestAnnounceService_SwitchedOffPublishesClosedOnce(t *testing.T) {
	ctx := context.Background()
	f := newAnnounceFixture(t, constAnswers(testAnswers(1, 2, 5)))
	f.run()
	require.Equal(t, "open", tagValue(f.relays.last(), "status"))

	f.setConfig(func(c *config.NodeConfig) { c.Announce.Enabled = false })
	f.run()
	require.Equal(t, 4, f.relays.count())
	assert.Equal(t, "closed", tagValue(f.relays.last(), "status"))
	st, _ := f.store.GetAnnounceState(ctx, f.node.NodeId)
	assert.False(t, st.OffClosedPublishedAt.IsZero())
	assert.Equal(t, core.AnnounceOpen, st.Status, "the stored status is untouched")

	// Once only, also after 8 days and after a restart.
	*f.now = f.now.Add(8 * 24 * time.Hour)
	f.run()
	restarted := NewAnnounceService(AnnounceServiceConfig{Relays: testRelays, Store: f.store, Publisher: f.relays, Now: func() time.Time { return *f.now }},
		[]AnnounceNode{f.svc.nodeList()[0].AnnounceNode})
	restarted.RunOnce(ctx)
	assert.Equal(t, 4, f.relays.count())

	// Switched on again: open again, mark cleared.
	f.setConfig(func(c *config.NodeConfig) { c.Announce.Enabled = true })
	f.run()
	require.Equal(t, 6, f.relays.count())
	assert.Equal(t, "open", tagValue(f.relays.last(), "status"))
	st, _ = f.store.GetAnnounceState(ctx, f.node.NodeId)
	assert.True(t, st.OffClosedPublishedAt.IsZero())

	// Unlisted, then switched off: already withdrawn, nothing more.
	_, _ = f.store.ApplyAnnounceAction(ctx, f.node.NodeId, core.AnnounceUnlist)
	f.run()
	require.Equal(t, 8, f.relays.count())
	f.setConfig(func(c *config.NodeConfig) { c.Announce.Enabled = false })
	f.run()
	assert.Equal(t, 8, f.relays.count())
}

func TestAnnounceService_SwitchedOffNeverPublishedSendsNothing(t *testing.T) {
	f := newAnnounceFixture(t, constAnswers(testAnswers(1)))
	f.setConfig(func(c *config.NodeConfig) { c.Announce.Enabled = false })
	f.run()
	assert.Equal(t, 0, f.relays.count())
	assert.Empty(t, f.status().LastError)
}

// A self-check block pauses the announcement, and never holds back a
// withdrawal.
func TestAnnounceService_BlockedNodeCanStillWithdraw(t *testing.T) {
	ctx := context.Background()
	f := newAnnounceFixture(t, constAnswers(testAnswers(1)))
	f.run()
	require.NoError(t, f.store.SetSelfCheckBlock(ctx, f.node.NodeId, "self-check failed: x"))
	f.setConfig(func(c *config.NodeConfig) { c.Announce.Title = "Renamed" })
	f.run()
	require.Equal(t, 4, f.relays.count())
	assert.Equal(t, "paused", tagValue(f.relays.last(), "status"), "blocked: announced as paused")

	f.setConfig(func(c *config.NodeConfig) { c.Announce.Enabled = false })
	f.run()
	require.Equal(t, 6, f.relays.count())
	assert.Equal(t, "closed", tagValue(f.relays.last(), "status"))
}

func TestBackoffDelay(t *testing.T) {
	base, max := 30*time.Second, 30*time.Minute
	want := []time.Duration{30 * time.Second, time.Minute, 2 * time.Minute, 4 * time.Minute, 8 * time.Minute, 16 * time.Minute, 30 * time.Minute, 30 * time.Minute}
	for i, w := range want {
		assert.Equal(t, w, backoffDelay(i+1, base, max), "failure %d", i+1)
	}
	assert.Equal(t, max, backoffDelay(1000, base, max))
}

// A relay that keeps refusing is tried at 30 s, 1 min, 2 min, ... up to
// 30 min apart; the others are not held back, and success resets it.
func TestAnnounceService_RelayBackoff(t *testing.T) {
	f := newAnnounceFixture(t, constAnswers(testAnswers(1)))
	bad := testRelays[0]
	f.relays.setReject(map[string]bool{bad: true})
	start := *f.now

	f.run() // try 1 at 0
	require.Equal(t, 1, f.relays.tries(bad))
	require.Equal(t, 1, f.relays.count(), "the good relay accepted")

	// Run every 10 s for 70 minutes and record when the bad relay is tried.
	var at []time.Duration
	last := 1
	for d := 10 * time.Second; d <= 70*time.Minute; d += 10 * time.Second {
		*f.now = start.Add(d)
		f.run()
		if n := f.relays.tries(bad); n != last {
			at = append(at, d)
			last = n
		}
	}
	assert.Equal(t, []time.Duration{
		30 * time.Second,                  // +30 s
		90 * time.Second,                  // +1 min
		210 * time.Second,                 // +2 min
		450 * time.Second,                 // +4 min
		930 * time.Second,                 // +8 min
		1890 * time.Second,                // +16 min
		1890*time.Second + 30*time.Minute, // capped at +30 min
	}, at)
	assert.Equal(t, 1, f.relays.count(), "the good relay got the event once")
	r := f.status().Last.Relays[0]
	assert.False(t, r.Accepted)
	assert.False(t, r.RetryAt.IsZero())

	// It accepts at its next try; its backoff resets.
	f.relays.setReject(map[string]bool{})
	*f.now = r.RetryAt
	f.run()
	assert.Equal(t, 2, f.relays.count())
	assert.True(t, f.status().Last.Relays[0].Accepted)
	assert.True(t, f.status().Last.Relays[0].RetryAt.IsZero())
	f2 := f.svc.nodeList()[0]
	assert.Empty(t, f2.backoff, "reset on success")
}

func TestNextCreatedAt(t *testing.T) {
	assert.Equal(t, int64(100), nextCreatedAt(100))
	assert.Equal(t, int64(100), nextCreatedAt(100, 99, 0))
	assert.Equal(t, int64(101), nextCreatedAt(100, 100, 0), "same second")
	assert.Equal(t, int64(151), nextCreatedAt(100, 150, 120), "clock went back")
	assert.Equal(t, int64(151), nextCreatedAt(100, 120, 150))
}

// F-M4: a status change in the same second as the event before it, and an
// event signed after a restart with the clock behind, are still newer.
func TestAnnounceService_CreatedAtIsMonotonic(t *testing.T) {
	ctx := context.Background()
	f := newAnnounceFixture(t, constAnswers(testAnswers(1, 2, 5)))
	f.run()
	first := f.relays.last()
	require.Equal(t, f.now.Unix(), int64(first.CreatedAt))

	// Pause and Resume within the same second.
	_, err := f.store.ApplyAnnounceAction(ctx, f.node.NodeId, core.AnnouncePause)
	require.NoError(t, err)
	f.run()
	paused := f.relays.last()
	assert.Equal(t, "paused", tagValue(paused, "status"))
	assert.Equal(t, first.CreatedAt+1, paused.CreatedAt)
	_, err = f.store.ApplyAnnounceAction(ctx, f.node.NodeId, core.AnnounceResume)
	require.NoError(t, err)
	f.run()
	open := f.relays.last()
	assert.Equal(t, "open", tagValue(open, "status"))
	assert.Equal(t, paused.CreatedAt+1, open.CreatedAt)
	st, _ := f.store.GetAnnounceState(ctx, f.node.NodeId)
	assert.Equal(t, int64(open.CreatedAt), st.LastCreatedAt, "stored")

	// Restarted with the clock an hour behind: still after the stored one.
	*f.now = f.now.Add(-time.Hour)
	restarted := NewAnnounceService(AnnounceServiceConfig{Relays: testRelays, Store: f.store, Publisher: f.relays, Now: func() time.Time { return *f.now }},
		[]AnnounceNode{f.svc.nodeList()[0].AnnounceNode})
	restarted.RunOnce(ctx)
	assert.Equal(t, open.CreatedAt+1, f.relays.last().CreatedAt)
	assert.Empty(t, restarted.Status(f.node.NodeId).LastError)
}

// F-M2: switching off withdraws the node even when the config or the live
// answers can no longer build an announcement: closed is built from the
// last announced values.
func TestAnnounceService_SwitchOffWithdrawsFromTheLastAnnouncedValues(t *testing.T) {
	cases := []struct {
		name string
		off  func(f *announceFixture, answers **ports.LSPSAnswers)
	}{
		{"announce block removed", func(f *announceFixture, _ **ports.LSPSAnswers) {
			c := *f.node.NodeConfig.Load()
			c.Announce = nil
			f.node.NodeConfig.Store(&c)
		}},
		{"disabled with addrs cleared", func(f *announceFixture, _ **ports.LSPSAnswers) {
			f.setConfig(func(c *config.NodeConfig) { c.Announce.Enabled = false; c.Announce.Addrs = nil })
		}},
		{"disabled with an empty LSPS2 menu", func(f *announceFixture, a **ports.LSPSAnswers) {
			*a = &ports.LSPSAnswers{Protocols: []int{2}}
			f.setConfig(func(c *config.NodeConfig) { c.Announce.Enabled = false })
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			answers := testAnswers(1, 2, 5)
			f := newAnnounceFixture(t, &fakeAnswers{next: func(int) (*ports.LSPSAnswers, error) { return answers, nil }})
			f.run()
			open := f.relays.last()
			require.Equal(t, "open", tagValue(open, "status"))

			tc.off(f, &answers)
			f.run()
			require.Equal(t, 4, f.relays.count())
			closed := f.relays.last()
			assert.Equal(t, "closed", tagValue(closed, "status"))
			assert.Equal(t, "203.0.113.7:9735", tagValue(closed, "addr"), "the last announced addr")
			assert.Equal(t, tagValue(open, "lsps2_fee"), tagValue(closed, "lsps2_fee"))
			ok, err := closed.CheckSignature()
			require.NoError(t, err)
			assert.True(t, ok)
			assert.Empty(t, f.status().LastError)
			st, _ := f.store.GetAnnounceState(context.Background(), f.node.NodeId)
			assert.False(t, st.OffClosedPublishedAt.IsZero())
		})
	}
}

// F-M2: an Unlist also withdraws when the live answers no longer build.
func TestAnnounceService_UnlistWithdrawsFromTheLastAnnouncedValues(t *testing.T) {
	answers := testAnswers(1, 2, 5)
	f := newAnnounceFixture(t, &fakeAnswers{next: func(int) (*ports.LSPSAnswers, error) { return answers, nil }})
	f.run()
	answers = &ports.LSPSAnswers{Protocols: []int{2}}
	_, err := f.store.ApplyAnnounceAction(context.Background(), f.node.NodeId, core.AnnounceUnlist)
	require.NoError(t, err)
	f.run()
	require.Equal(t, 4, f.relays.count())
	assert.Equal(t, "closed", tagValue(f.relays.last(), "status"))
}
