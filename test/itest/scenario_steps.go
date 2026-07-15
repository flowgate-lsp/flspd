package itest

import (
	"encoding/hex"
	"fmt"
	"testing"
	"time"

	"github.com/flokiorg/flnd/lnrpc"
	"github.com/flowgate-lsp/flspd/lsps"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func init() {
	stepRegistry["connect_peer"] = stepConnectPeer
	stepRegistry["open_channel"] = stepOpenChannel
	stepRegistry["mine_blocks"] = stepMineBlocks
	stepRegistry["auto_accept_channels"] = stepAutoAcceptChannels
	stepRegistry["lsps2_buy"] = stepLsps2Buy
	stepRegistry["create_jit_invoice"] = stepCreateJitInvoice
	stepRegistry["pay_invoice"] = stepPayInvoice
}

// p2pAddr maps a devnet role name to its container-internal P2P address --
// peer connections happen over the devnet's own docker network, not the
// host-published port a test process would dial for gRPC.
func p2pAddr(role string) string {
	return fmt.Sprintf("flnd-%s:9735", role)
}

// --- connect_peer: {from: <role>, to: <role>} ---

type connectPeerParams struct {
	From string `yaml:"from"`
	To   string `yaml:"to"`
}

func stepConnectPeer(t *testing.T, sc *ScenarioContext, params *yaml.Node) error {
	var p connectPeerParams
	if err := decodeParamsStrict(params, &p); err != nil {
		return err
	}
	if p.From == "" || p.To == "" {
		return fmt.Errorf("connect_peer: both from and to are required")
	}

	fromRaw, err := sc.raw(p.From)
	if err != nil {
		return err
	}
	toPubkey, err := sc.pubkey(p.To)
	if err != nil {
		return err
	}
	if err := ConnectPeer(sc.Ctx, fromRaw, toPubkey, p2pAddr(p.To)); err != nil {
		return fmt.Errorf("connecting %s -> %s: %w", p.From, p.To, err)
	}

	fromClient, err := sc.client(p.From)
	if err != nil {
		return err
	}
	toPubkeyBytes, err := hex.DecodeString(toPubkey)
	if err != nil {
		return err
	}
	require.Eventually(t, func() bool {
		connected, err := fromClient.IsConnected(toPubkeyBytes)
		return err == nil && connected
	}, 20*time.Second, 500*time.Millisecond, "%s must connect to %s", p.From, p.To)
	return nil
}

// --- open_channel: {from: <role>, to: <role>, capacity_loki: N, min_confs: N} ---
// Idempotent: reuses an existing confirmed from->to channel rather than
// opening a redundant one, since this devnet's chain/peer state is
// long-lived across runs (see test/devnet/README.md) -- piling up
// duplicate channels against the same peer was observed to make
// pathfinding pick the newest, not-yet-graph-synced one and fail with
// NO_ROUTE.

type openChannelParams struct {
	From         string `yaml:"from"`
	To           string `yaml:"to"`
	CapacityLoki int64  `yaml:"capacity_loki"`
	MinConfs     int32  `yaml:"min_confs"`
}

func stepOpenChannel(t *testing.T, sc *ScenarioContext, params *yaml.Node) error {
	var p openChannelParams
	if err := decodeParamsStrict(params, &p); err != nil {
		return err
	}
	if p.From == "" || p.To == "" || p.CapacityLoki == 0 {
		return fmt.Errorf("open_channel: from, to, and capacity_loki are all required")
	}
	if p.MinConfs == 0 {
		p.MinConfs = 1
	}

	fromClient, err := sc.client(p.From)
	if err != nil {
		return err
	}
	toPubkeyHex, err := sc.pubkey(p.To)
	if err != nil {
		return err
	}

	channels, err := fromClient.ListChannels()
	if err != nil {
		return err
	}
	for _, ch := range channels {
		if hex.EncodeToString(ch.PeerId) == toPubkeyHex && ch.ConfirmedScid != nil {
			return nil // already have a confirmed channel; nothing to do
		}
	}

	fromRaw, err := sc.raw(p.From)
	if err != nil {
		return err
	}
	if _, err := OpenNormalChannel(sc.Ctx, fromRaw, toPubkeyHex, p.CapacityLoki, p.MinConfs); err != nil {
		return fmt.Errorf("opening channel %s -> %s: %w", p.From, p.To, err)
	}

	require.Eventually(t, func() bool {
		resp, err := fromRaw.ListChannels(sc.Ctx, &lnrpc.ListChannelsRequest{})
		if err != nil {
			return false
		}
		for _, ch := range resp.Channels {
			if ch.RemotePubkey == toPubkeyHex && ch.Active {
				return true
			}
		}
		return false
	}, 30*time.Second, time.Second, "%s<->%s channel must confirm and become active", p.From, p.To)

	return nil
}

// --- mine_blocks: {count: N} ---

type mineBlocksParams struct {
	Count int `yaml:"count"`
}

func stepMineBlocks(t *testing.T, sc *ScenarioContext, params *yaml.Node) error {
	var p mineBlocksParams
	if err := decodeParamsStrict(params, &p); err != nil {
		return err
	}
	if p.Count <= 0 {
		return fmt.Errorf("mine_blocks: count must be positive")
	}
	return sc.Devnet.MineBlocks(p.Count)
}

// --- auto_accept_channels: {as: <role>} ---
// flnd (like upstream LND) rejects every incoming channel-open request
// outright unless the responder has registered a ChannelAcceptor.

type autoAcceptChannelsParams struct {
	As string `yaml:"as"`
}

func stepAutoAcceptChannels(t *testing.T, sc *ScenarioContext, params *yaml.Node) error {
	var p autoAcceptChannelsParams
	if err := decodeParamsStrict(params, &p); err != nil {
		return err
	}
	if p.As == "" {
		return fmt.Errorf("auto_accept_channels: as is required")
	}
	raw, err := sc.raw(p.As)
	if err != nil {
		return err
	}
	return AutoAcceptChannels(sc.Ctx, raw)
}

// --- lsps2_buy: {as: <role>, lsp: <role>, token: str, payment_size_mloki: N, save_as: name} ---

type lsps2BuyParams struct {
	As               string `yaml:"as"`
	Lsp              string `yaml:"lsp"`
	Token            string `yaml:"token"`
	PaymentSizeMloki uint64 `yaml:"payment_size_mloki"`
	SaveAs           string `yaml:"save_as"`
}

// lsps2BuySaved bundles what later steps need to build/pay the JIT invoice.
type lsps2BuySaved struct {
	Result           *lsps.Lsps2BuyResult
	PaymentSizeMloki uint64
}

func stepLsps2Buy(t *testing.T, sc *ScenarioContext, params *yaml.Node) error {
	var p lsps2BuyParams
	if err := decodeParamsStrict(params, &p); err != nil {
		return err
	}
	if p.As == "" || p.Lsp == "" || p.PaymentSizeMloki == 0 || p.SaveAs == "" {
		return fmt.Errorf("lsps2_buy: as, lsp, payment_size_mloki, and save_as are all required")
	}

	buyerRaw, err := sc.raw(p.As)
	if err != nil {
		return err
	}
	lspPubkey, err := sc.pubkey(p.Lsp)
	if err != nil {
		return err
	}

	bus, err := NewCustomMessageBus(sc.Ctx, buyerRaw)
	if err != nil {
		return fmt.Errorf("building custom message bus: %w", err)
	}
	client, err := NewLsps2Client(bus, lspPubkey)
	if err != nil {
		return err
	}
	menu, err := client.GetInfo(sc.Ctx, p.Token)
	if err != nil {
		return fmt.Errorf("lsps2 get_info: %w", err)
	}
	if len(menu) == 0 {
		return fmt.Errorf("lsps2 get_info: LSP offered no opening_fee_params options")
	}
	result, err := client.Buy(sc.Ctx, menu[0], p.PaymentSizeMloki)
	if err != nil {
		return fmt.Errorf("lsps2 buy: %w", err)
	}
	return sc.save(p.SaveAs, &lsps2BuySaved{Result: result, PaymentSizeMloki: p.PaymentSizeMloki})
}

// --- create_jit_invoice: {as: <role>, from: <save_as ref>, memo: str, save_as: name} ---

type createJitInvoiceParams struct {
	As     string `yaml:"as"`
	From   string `yaml:"from"`
	Memo   string `yaml:"memo"`
	SaveAs string `yaml:"save_as"`
}

func stepCreateJitInvoice(t *testing.T, sc *ScenarioContext, params *yaml.Node) error {
	var p createJitInvoiceParams
	if err := decodeParamsStrict(params, &p); err != nil {
		return err
	}
	if p.As == "" || p.From == "" || p.SaveAs == "" {
		return fmt.Errorf("create_jit_invoice: as, from, and save_as are all required")
	}

	saved, err := sc.get(p.From)
	if err != nil {
		return err
	}
	buy, ok := saved.(*lsps2BuySaved)
	if !ok {
		return fmt.Errorf("create_jit_invoice: %q was not saved by lsps2_buy", p.From)
	}

	asRaw, err := sc.raw(p.As)
	if err != nil {
		return err
	}
	bolt11, _, err := CreateJitInvoice(sc.Ctx, asRaw, buy.Result, buy.PaymentSizeMloki, p.Memo)
	if err != nil {
		return fmt.Errorf("creating jit invoice: %w", err)
	}
	return sc.save(p.SaveAs, bolt11)
}

// --- pay_invoice: {as: <role>, invoice: <save_as ref>, outgoing_channel_to: <role>, timeout_seconds: N} ---

type payInvoiceParams struct {
	As                string `yaml:"as"`
	Invoice           string `yaml:"invoice"`
	OutgoingChannelTo string `yaml:"outgoing_channel_to"`
	TimeoutSeconds    int    `yaml:"timeout_seconds"`
}

func stepPayInvoice(t *testing.T, sc *ScenarioContext, params *yaml.Node) error {
	var p payInvoiceParams
	if err := decodeParamsStrict(params, &p); err != nil {
		return err
	}
	if p.As == "" || p.Invoice == "" || p.TimeoutSeconds == 0 {
		return fmt.Errorf("pay_invoice: as, invoice, and timeout_seconds are all required")
	}

	saved, err := sc.get(p.Invoice)
	if err != nil {
		return err
	}
	bolt11, ok := saved.(string)
	if !ok {
		return fmt.Errorf("pay_invoice: %q was not saved as an invoice string", p.Invoice)
	}

	var outgoingChanId uint64
	if p.OutgoingChannelTo != "" {
		toPubkeyHex, err := sc.pubkey(p.OutgoingChannelTo)
		if err != nil {
			return err
		}
		asClient, err := sc.client(p.As)
		if err != nil {
			return err
		}
		channels, err := asClient.ListChannels()
		if err != nil {
			return err
		}
		for _, ch := range channels {
			if hex.EncodeToString(ch.PeerId) == toPubkeyHex && ch.ConfirmedScid != nil {
				outgoingChanId = uint64(*ch.ConfirmedScid)
				break
			}
		}
		if outgoingChanId == 0 {
			return fmt.Errorf("pay_invoice: no confirmed channel from %s to %s to pin as the outgoing channel", p.As, p.OutgoingChannelTo)
		}
	}

	router, err := sc.router(p.As)
	if err != nil {
		return err
	}
	_, err = PayInvoice(sc.Ctx, router, bolt11, outgoingChanId, time.Duration(p.TimeoutSeconds)*time.Second)
	return err
}
