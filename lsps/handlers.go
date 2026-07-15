package lsps

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/flokiorg/flnd/lnrpc"
	"github.com/flowgate-lsp/flspd/core"
	"github.com/flowgate-lsp/flspd/ports"
)

// LndClientWrapper is the subset of the lnd client Handlers itself needs
// directly -- issuing the bolt11 invoice an LSPS1 order is paid against.
// The message-transport methods (SubscribeCustomMessages/SendCustomMessage)
// live on transport/lsps's own, narrower interface instead: Handlers never
// touches the wire, only what a JSON-RPC method handler needs to do its
// job.
type LndClientWrapper interface {
	AddInvoice(ctx context.Context, req *lnrpc.Invoice) (*lnrpc.AddInvoiceResponse, error)
}

// NotificationService interface for sending LSPS5 webhook notifications
type NotificationService interface {
	NotifyWebhookRegistered(pubkey string, webhook *core.Lsps5Webhook) bool
	NotifyOrderStateChanged(pubkey string, orderId string, state string, channelPoint string, errorMsg string) (bool, error)
}

type Config struct {
	Lsps1MaxPendingOrders int
	Lsps5MaxWebhooks      int
	EnableLsps5Https      bool   // Enable HTTPS webhook transport
	EnableLsps5Nostr      bool   // Enable Nostr notification transport
	Lsps5NostrPubkey      string // Hex-encoded Nostr public key for notifications
}

// Handlers implements the LSPS0/1/2/5 protocol methods: parsing a
// method's params, applying its business rules, and returning a result or
// error -- everything downstream of "a JSON-RPC request already arrived
// addressed to this method name." How that request arrived (the
// lnd-custom-message subscription loop, JSON-RPC envelope parsing, method
// dispatch) is transport/lsps's job, not this type's; see Route below for
// the one seam between them.
type Handlers struct {
	config              Config
	lndClient           LndClientWrapper
	nodesService        ports.NodesService
	openingService      ports.OpeningService
	liquidityManager    LiquidityManager
	notificationService NotificationService
}

func NewHandlers(
	config Config,
	lndClient LndClientWrapper,
	nodesService ports.NodesService,
	openingService ports.OpeningService,
	liquidityManager LiquidityManager,
	notificationService NotificationService,
) *Handlers {
	return &Handlers{
		config:              config,
		lndClient:           lndClient,
		nodesService:        nodesService,
		openingService:      openingService,
		liquidityManager:    liquidityManager,
		notificationService: notificationService,
	}
}

// JSON-RPC Types
type JsonRpcRequest struct {
	JsonRpc string          `json:"jsonrpc"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params"`
	Id      interface{}     `json:"id"`
}

type JsonRpcResponse struct {
	JsonRpc string        `json:"jsonrpc"`
	Result  interface{}   `json:"result,omitempty"`
	Error   *JsonRpcError `json:"error,omitempty"`
	Id      interface{}   `json:"id"`
}

type JsonRpcError struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

func NewJsonRpcResponse(id interface{}, result interface{}) *JsonRpcResponse {
	return &JsonRpcResponse{
		JsonRpc: "2.0",
		Id:      id,
		Result:  result,
	}
}

func NewJsonRpcErrorResponse(id interface{}, code int, message string, data interface{}) *JsonRpcResponse {
	// For LSPS0 compatibility, -32602 (Invalid params) must include an 'unrecognized' array.
	if code == -32602 && data == nil {
		data = map[string]interface{}{
			"unrecognized": []string{},
		}
	}

	return &JsonRpcResponse{
		JsonRpc: "2.0",
		Id:      id,
		Error: &JsonRpcError{
			Code:    code,
			Message: message,
			Data:    data,
		},
	}
}

// Route dispatches an already-decoded JSON-RPC request to the handler for
// its method, by name. The only seam between transport/lsps (which owns
// the lnd-custom-message subscription loop and JSON-RPC envelope
// marshal/unmarshal) and Handlers (this type): transport/lsps calls Route
// once it has a *JsonRpcRequest, and does nothing else with the method
// name itself.
func (s *Handlers) Route(req *JsonRpcRequest, peer []byte) *JsonRpcResponse {
	switch req.Method {
	case "lsps0.list_protocols":
		return s.handleLsps0ListProtocols(req)
	case "lsps0.get_info":
		return s.handleLsps0GetInfo(req)
	case "lsps2.get_versions":
		return s.handleLsps2GetVersions(req)
	case "lsps2.get_info":
		return s.lsps2GetInfo(req)
	case "lsps2.buy":
		return s.lsps2Buy(req, peer)
	case "lsps1.get_info":
		return s.handleLsps1GetInfo(req)
	case "lsps1.create_order":
		return s.handleLsps1CreateOrder(req, peer)
	case "lsps1.get_order":
		return s.handleLsps1GetOrder(req)
	case "lsps5.set_webhook":
		return s.handleLsps5SetWebhook(req, peer)
	case "lsps5.list_webhooks":
		return s.handleLsps5ListWebhooks(req, peer)
	case "lsps5.remove_webhook":
		return s.handleLsps5RemoveWebhook(req, peer)
	default:
		return NewJsonRpcErrorResponse(req.Id, -32601, "Method not found", nil)
	}
}

func generateRandomScid() (uint64, error) {
	b := make([]byte, 8)
	_, err := rand.Read(b)
	if err != nil {
		return 0, err
	}
	raw := binary.BigEndian.Uint64(b)

	// Phantom SCIDs must be in LND's alias block-height range [16,000,000, 16,250,000)
	// so that XAddLocalChanAliases can register them after a JIT channel opens,
	// allowing the switch to route the resumed HTLC to the newly opened channel.
	const aliasStart = uint64(16_000_000)
	const aliasRange = uint64(250_000)

	blockHeight := aliasStart + (raw>>40)%aliasRange
	lower40 := raw & 0xFF_FFFF_FFFF // 24-bit tx index + 16-bit output index

	return (blockHeight << 40) | lower40, nil
}

func generateUUID() (string, error) {
	b := make([]byte, 16)
	_, err := rand.Read(b)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:]), nil
}

func parseUint64(s string) uint64 {
	v, _ := strconv.ParseUint(s, 10, 64)
	return v
}

// ComputeOpeningFee is the single implementation of the LSPS1/LSPS2
// channel-opening fee (F-5): proportional * paymentSizeMloki, rounded up
// to the nearest mloki, floored at minFeeMloki. Exported so callers
// outside this package (e.g. the `flspd config explain fees` CLI) can
// reuse the exact same math flspd itself uses to quote a fee, rather than
// re-deriving it.
func ComputeOpeningFee(paymentSizeMloki, minFeeMloki, proportional uint64) (uint64, error) {
	var prod uint64
	if paymentSizeMloki != 0 {
		prod = paymentSizeMloki * proportional
		if prod/paymentSizeMloki != proportional {
			return 0, fmt.Errorf("overflow in multiplication")
		}
	} else {
		prod = 0
	}

	sum := prod + 999999
	if sum < prod {
		return 0, fmt.Errorf("overflow in addition")
	}

	openingFee := sum / 1000000

	if openingFee < minFeeMloki {
		openingFee = minFeeMloki
	}
	return openingFee, nil
}

func formatScid(scid uint64) string {
	block := scid >> 40
	tx := (scid >> 16) & 0xFFFFFF
	output := scid & 0xFFFF
	return fmt.Sprintf("%dx%dx%d", block, tx, output)
}
