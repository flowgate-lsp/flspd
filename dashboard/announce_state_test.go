package dashboard

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/flowgate-lsp/flspd/core"
	"github.com/flowgate-lsp/flspd/lsps"
	"github.com/flowgate-lsp/flspd/platform/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeAnnounceStore is an in-memory ports.AnnounceStateStore using the real
// transition rules.
type fakeAnnounceStore struct {
	states map[string]*core.AnnounceState
	err    error
}

func (f *fakeAnnounceStore) GetAnnounceState(_ context.Context, nodeID []byte) (*core.AnnounceState, error) {
	if f.err != nil {
		return nil, f.err
	}
	if st, ok := f.states[string(nodeID)]; ok {
		return st, nil
	}
	return &core.AnnounceState{NodeID: nodeID, Status: core.AnnounceOpen}, nil
}

func (f *fakeAnnounceStore) ApplyAnnounceAction(ctx context.Context, nodeID []byte, a core.AnnounceAction) (*core.AnnounceState, error) {
	cur, err := f.GetAnnounceState(ctx, nodeID)
	if err != nil {
		return nil, err
	}
	next, err := core.NextAnnounceStatus(cur.Status, a)
	if err != nil {
		return nil, err
	}
	st := &core.AnnounceState{NodeID: nodeID, Status: next, UpdatedAt: time.Unix(1792713600, 0)}
	f.states[string(nodeID)] = st
	return st, nil
}

func announceStateHandlers(t *testing.T, announcing bool) (*Handlers, *fakeAnnounceStore, *[]*core.AnnounceState) {
	t.Helper()
	cfg := &config.NodeConfig{Name: "Node A"}
	if announcing {
		cfg.Announce = &config.AnnounceConfig{Enabled: true, Addrs: []string{"203.0.113.7:9735"}}
	}
	node := &core.Node{NodeId: []byte{0x02, 0xaa}, NodeConfig: config.NewNodeConfigRef(cfg)}
	mockNodes := new(MockNodesService)
	mockNodes.On("GetNodes").Return([]*core.Node{node})

	s := NewHandlers(mockNodes, nil, nil, nil, "")
	store := &fakeAnnounceStore{states: map[string]*core.AnnounceState{}}
	var changes []*core.AnnounceState
	s.SetAnnounceState(store, func(st *core.AnnounceState) { changes = append(changes, st) })
	s.SetAnnounce([]string{"wss://relay.example.com"}, nil)
	return s, store, &changes
}

func call(t *testing.T, s *Handlers, method, params string) (*AnnounceStateResponse, *lsps.JsonRpcError) {
	t.Helper()
	req := &lsps.JsonRpcRequest{Id: json.RawMessage(`1`), Method: method}
	if params != "" {
		req.Params = json.RawMessage(params)
	}
	resp := s.Route(req)
	require.NotNil(t, resp)
	if resp.Error != nil {
		return nil, resp.Error
	}
	out, ok := resp.Result.(AnnounceStateResponse)
	require.True(t, ok)
	return &out, nil
}

func TestAnnounceState_GetDefault(t *testing.T) {
	s, _, _ := announceStateHandlers(t, true)
	got, rpcErr := call(t, s, "get_announce_state", "")
	require.Nil(t, rpcErr)
	assert.Equal(t, "open", got.Status)
	assert.True(t, got.Enabled)
	assert.Nil(t, got.UpdatedAt)
	assert.Equal(t, "02aa", got.NodePubkey)
	assert.Equal(t, []string{"wss://relay.example.com"}, got.Relays)

	raw, err := json.Marshal(got)
	require.NoError(t, err)
	assert.Contains(t, string(raw), `"updated_at":null`)
}

func TestAnnounceState_PauseResumeUnlist(t *testing.T) {
	s, _, changes := announceStateHandlers(t, true)

	got, rpcErr := call(t, s, "set_announce_state", `{"action":"pause"}`)
	require.Nil(t, rpcErr)
	assert.Equal(t, "paused", got.Status)
	require.NotNil(t, got.UpdatedAt)
	assert.Equal(t, "2026-10-23T00:00:00Z", *got.UpdatedAt)

	got, _ = call(t, s, "get_announce_state", "")
	assert.Equal(t, "paused", got.Status)

	got, rpcErr = call(t, s, "set_announce_state", `{"action":"unlist"}`)
	require.Nil(t, rpcErr)
	assert.Equal(t, "closed", got.Status)

	_, rpcErr = call(t, s, "set_announce_state", `{"action":"pause"}`)
	require.NotNil(t, rpcErr, "pause is refused while unlisted")
	assert.Equal(t, -32602, rpcErr.Code)

	got, rpcErr = call(t, s, "set_announce_state", `{"action":"resume"}`)
	require.Nil(t, rpcErr)
	assert.Equal(t, "open", got.Status)

	statuses := []core.AnnounceStatus{}
	for _, c := range *changes {
		statuses = append(statuses, c.Status)
	}
	assert.Equal(t, []core.AnnounceStatus{core.AnnouncePaused, core.AnnounceClosed, core.AnnounceOpen}, statuses,
		"the hook sees every applied action, not refused ones")
}

func TestAnnounceState_BadRequests(t *testing.T) {
	s, _, changes := announceStateHandlers(t, false)
	for _, params := range []string{`{"action":"delete"}`, `{}`, `not-json`, `{"action":"open"}`} {
		_, rpcErr := call(t, s, "set_announce_state", params)
		require.NotNil(t, rpcErr, params)
		assert.Equal(t, -32602, rpcErr.Code, params)
	}
	assert.Empty(t, *changes)

	got, _ := call(t, s, "get_announce_state", "")
	assert.False(t, got.Enabled, "state is readable while announcing is off")
}

func TestAnnounceState_StoreErrorsAndUnavailable(t *testing.T) {
	s, store, _ := announceStateHandlers(t, true)
	store.err = errors.New("db down")
	_, rpcErr := call(t, s, "get_announce_state", "")
	require.NotNil(t, rpcErr)
	assert.Equal(t, -32603, rpcErr.Code)
	assert.NotContains(t, rpcErr.Message, "db down", "internal errors are not leaked")

	bare := NewHandlers(new(MockNodesService), nil, nil, nil, "")
	_, rpcErr = call(t, bare, "get_announce_state", "")
	require.NotNil(t, rpcErr)
	assert.Equal(t, -32000, rpcErr.Code)
}
