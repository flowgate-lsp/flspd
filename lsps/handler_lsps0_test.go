package lsps

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHandleLsps0ListProtocols(t *testing.T) {
	server := &Handlers{}
	req := &JsonRpcRequest{Id: json.RawMessage(`"list-protocols-id"`), Method: "lsps0.list_protocols"}

	resp := server.handleLsps0ListProtocols(req)

	require.NotNil(t, resp)
	assert.Nil(t, resp.Error)
	result, ok := resp.Result.(map[string][]int)
	require.True(t, ok)
	assert.Equal(t, []int{1, 2, 5}, result["protocols"])
}

func TestHandleLsps0GetInfo_WithoutNostrPubkey(t *testing.T) {
	server := &Handlers{config: Config{}}
	req := &JsonRpcRequest{Id: json.RawMessage(`"get-info-id"`), Method: "lsps0.get_info"}

	resp := server.handleLsps0GetInfo(req)

	require.NotNil(t, resp)
	assert.Nil(t, resp.Error)
	result, ok := resp.Result.(Lsps0GetInfoResponse)
	require.True(t, ok)
	assert.Equal(t, []int{1, 2, 5}, result.SupportedVersions)
	assert.Empty(t, result.NotificationNostrPubkey, "must omit the nostr pubkey field when none is configured")
}

func TestHandleLsps0GetInfo_WithNostrPubkey(t *testing.T) {
	server := &Handlers{config: Config{Lsps5NostrPubkey: "deadbeef"}}
	req := &JsonRpcRequest{Id: json.RawMessage(`"get-info-id"`), Method: "lsps0.get_info"}

	resp := server.handleLsps0GetInfo(req)

	require.NotNil(t, resp)
	result, ok := resp.Result.(Lsps0GetInfoResponse)
	require.True(t, ok)
	assert.Equal(t, "deadbeef", result.NotificationNostrPubkey)
}
