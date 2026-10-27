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

func (m *memAnnounceStore) MarkPublished(_ context.Context, id []byte, at time.Time) error {
	return m.update(id, func(st *core.AnnounceState) { st.LastPublishedAt = at })
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
	mu     sync.Mutex
	events []nostr.Event
	reject map[string]bool
}

func (f *fakeRelays) Publish(_ context.Context, url string, ev nostr.Event) error {
	f.mu.Lock()
	defer f.mu.Unlock()
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

func TestAnnounceService_ClosedRetriedUntilARelayAccepts(t *testing.T) {
	f := newAnnounceFixture(t, constAnswers(testAnswers(1)))
	_, _ = f.store.ApplyAnnounceAction(context.Background(), f.node.NodeId, core.AnnounceUnlist)
	f.relays.reject = map[string]bool{testRelays[0]: true, testRelays[1]: true}
	f.run()
	st, _ := f.store.GetAnnounceState(context.Background(), f.node.NodeId)
	assert.True(t, st.ClosedPublishedAt.IsZero(), "not recorded while no relay accepted")
	assert.Contains(t, f.status().LastError, "no relay accepted")

	f.relays.reject = map[string]bool{testRelays[0]: true}
	f.run()
	st, _ = f.store.GetAnnounceState(context.Background(), f.node.NodeId)
	assert.False(t, st.ClosedPublishedAt.IsZero())
	assert.Empty(t, f.status().LastError)
	rel := f.status().Last.Relays
	assert.False(t, rel[0].Accepted)
	assert.Contains(t, rel[0].Error, "blocked")
	assert.True(t, rel[1].Accepted)
}

func TestAnnounceService_FailedPublishIsRetried(t *testing.T) {
	f := newAnnounceFixture(t, constAnswers(testAnswers(1, 5)))
	f.relays.reject = map[string]bool{testRelays[0]: true, testRelays[1]: true}
	f.run()
	assert.Equal(t, 0, f.relays.count())
	assert.NotEmpty(t, f.status().LastError)

	f.relays.reject = map[string]bool{}
	f.run()
	assert.Equal(t, 2, f.relays.count(), "the unchanged values are published once a relay accepts")
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

// T8-4: a published announcement that does not match the live answers,
// while those answers did not change, blocks publishing until Recheck.
func TestAnnounceService_SelfCheckBlocksUntilRecheck(t *testing.T) {
	broken := false
	src := &fakeAnswers{next: func(call int) (*ports.LSPSAnswers, error) {
		if broken && call%2 == 0 {
			// The self-check's read: protocol 2's live answer is unreadable.
			a := testAnswers(1, 2, 5)
			a.LSPS2Info = json.RawMessage(`{"opening_fee_params_menu":"oops"}`)
			return a, nil
		}
		return testAnswers(1, 2, 5), nil
	}}
	f := newAnnounceFixture(t, src)
	broken = true
	f.run()
	require.Equal(t, 2, f.relays.count())
	st := f.status()
	require.NotNil(t, st.SelfCheck)
	assert.False(t, st.SelfCheck.OK)
	assert.Contains(t, st.Blocked, "self-check failed")

	// Blocked: a change does not publish.
	f.setConfig(func(c *config.NodeConfig) { c.Announce.Title = "Renamed" })
	f.run()
	assert.Equal(t, 2, f.relays.count())

	// Fixed and rechecked: publishing resumes and the check passes.
	broken = false
	assert.True(t, f.svc.Recheck(f.node.NodeId))
	f.run()
	assert.Equal(t, 4, f.relays.count())
	st = f.status()
	assert.Empty(t, st.Blocked)
	assert.True(t, st.SelfCheck.OK, "%v", st.SelfCheck.Problems)
	assert.False(t, f.svc.Recheck([]byte{1}), "unknown node")
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
