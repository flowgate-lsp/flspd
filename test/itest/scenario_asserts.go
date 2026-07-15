package itest

import (
	"encoding/hex"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func init() {
	assertRegistry["channel_exists"] = assertChannelExists
}

// --- channel_exists: {between: [<role>, <role>]} ---
// Asserts a channel currently exists between the two named roles (checked
// from the first role's own ListChannels), regardless of confirmation
// state -- callers wanting to assert *confirmed* specifically should check
// that as part of a step instead, since "exists" here also covers the
// zero-conf case where a real ConfirmedScid may not land for a while.

type channelExistsParams struct {
	Between []string `yaml:"between"`
}

func assertChannelExists(t *testing.T, sc *ScenarioContext, params *yaml.Node) {
	var p channelExistsParams
	if err := decodeParamsStrict(params, &p); err != nil {
		require.NoError(t, err)
		return
	}
	if len(p.Between) != 2 {
		t.Errorf("channel_exists: between must list exactly two roles, got %v", p.Between)
		return
	}

	from, to := p.Between[0], p.Between[1]
	fromClient, err := sc.client(from)
	if !assert.NoError(t, err) {
		return
	}
	toPubkeyHex, err := sc.pubkey(to)
	if !assert.NoError(t, err) {
		return
	}

	channels, err := fromClient.ListChannels()
	if !assert.NoError(t, err) {
		return
	}
	for _, ch := range channels {
		if hex.EncodeToString(ch.PeerId) == toPubkeyHex {
			return
		}
	}
	t.Errorf("channel_exists: no channel found between %s and %s", from, to)
}
