package lsps

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/flokiorg/flnd/lnrpc"
	"github.com/flowgate-lsp/flspd/common"
	"github.com/rs/zerolog/log"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const LspsMessageId = 51610

type LndClientWrapper interface {
	SubscribeCustomMessages(ctx context.Context, req *lnrpc.SubscribeCustomMessagesRequest) (lnrpc.Lightning_SubscribeCustomMessagesClient, error)
	SendCustomMessage(ctx context.Context, msg *lnrpc.SendCustomMessageRequest) (*lnrpc.SendCustomMessageResponse, error)
	AddInvoice(ctx context.Context, req *lnrpc.Invoice) (*lnrpc.AddInvoiceResponse, error)
}

// NotificationService interface for sending LSPS5 webhook notifications
type NotificationService interface {
	NotifyWebhookRegistered(pubkey string, webhook *common.Lsps5Webhook) bool
}

type Config struct {
	Lsps1MaxPendingOrders int
	Lsps5MaxWebhooks      int
	EnableLsps5Https      bool // Enable HTTPS webhook transport
	EnableLsps5Nostr      bool // Enable Nostr notification transport
}

type Server struct {
	config              Config
	lndClient           LndClientWrapper
	nodesService        common.NodesService
	openingService      common.OpeningService
	liquidityManager    LiquidityManager
	notificationService NotificationService

	quit     chan struct{}
	quitOnce sync.Once
	wg       sync.WaitGroup
}

func NewServer(
	config Config,
	lndClient LndClientWrapper,
	nodesService common.NodesService,
	openingService common.OpeningService,
	liquidityManager LiquidityManager,
	notificationService NotificationService,
) *Server {
	return &Server{
		config:              config,
		lndClient:           lndClient,
		nodesService:        nodesService,
		openingService:      openingService,
		liquidityManager:    liquidityManager,
		notificationService: notificationService,
		quit:                make(chan struct{}),
	}
}

func (s *Server) Start() {
	s.wg.Add(1)
	go s.messageLoop()
}

func (s *Server) Stop() {
	s.quitOnce.Do(func() {
		close(s.quit)
	})
	s.wg.Wait()
}

func (s *Server) messageLoop() {
	defer s.wg.Done()

	for {
		select {
		case <-s.quit:
			return
		default:
		}

		err := s.subscribeAndListen()
		if err != nil {
			log.Error().
				Err(err).
				Str("component", string(common.LogComponentLSPS0)).
				Msg("LSPS: Connection lost, retrying in 5s")
			select {
			case <-s.quit:
				return
			case <-time.After(5 * time.Second):
				continue
			}
		}
	}
}

func (s *Server) subscribeAndListen() error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Monitor quit channel to cancel context
	go func() {
		<-s.quit
		cancel()
	}()

	stream, err := s.lndClient.SubscribeCustomMessages(ctx, &lnrpc.SubscribeCustomMessagesRequest{})
	if err != nil {
		return fmt.Errorf("failed to subscribe: %w", err)
	}

	log.Info().
		Str("component", string(common.LogComponentLSPS0)).
		Msg("LSPS: Service started, listening for custom messages")

	for {
		msg, err := stream.Recv()
		if err != nil {
			if status.Code(err) == codes.Canceled || ctx.Err() != nil {
				return nil // Normal shutdown
			}
			return fmt.Errorf("recv error: %w", err)
		}

		if msg.Type == LspsMessageId {
			// Process in a goroutine to not block the receiver
			go s.handleMessage(msg)
		}
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

func (s *Server) handleMessage(msg *lnrpc.CustomMessage) {
	var req JsonRpcRequest
	err := json.Unmarshal(msg.Data, &req)
	if err != nil {
		log.Warn().
			Err(err).
			Str("component", string(common.LogComponentLSPS0)).
			Str("peer", fmt.Sprintf("%x", msg.Peer)).
			Msg("LSPS: Failed to parse incoming JSON-RPC request")
		// Cannot reply if we can't parse the ID, so just ignore valid malformed JSON as per spec
		// (Spec says: if parse error, respond with parse error but we need to know it's a valid JSON object first...
		// Actually spec says "In case of a bad message format error ... the LSP MUST respond with a JSON-RPC 2.0 error response")
		// But if we can't even unmarshal it, we might try to send a generic error if we can construct a response.
		// For now, logging is safer to avoid loops with bad peers.
		return
	}

	response := s.route(&req, msg.Peer)

	// Send Response
	respBytes, err := json.Marshal(response)
	if err != nil {
		log.Error().
			Err(err).
			Str("component", string(common.LogComponentLSPS0)).
			Msg("LSPS: Failed to marshal JSON-RPC response")
		return
	}

	_, err = s.lndClient.SendCustomMessage(context.Background(), &lnrpc.SendCustomMessageRequest{
		Peer: msg.Peer,
		Type: LspsMessageId,
		Data: respBytes,
	})
	if err != nil {
		log.Error().
			Err(err).
			Str("component", string(common.LogComponentLSPS0)).
			Msg("LSPS: Failed to send custom message to peer")
	}
}

func (s *Server) route(req *JsonRpcRequest, peer []byte) *JsonRpcResponse {
	switch req.Method {
	case "lsps0.list_protocols":
		return s.handleLsps0ListProtocols(req)
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
	return binary.BigEndian.Uint64(b), nil
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

func computeOpeningFee(paymentSizeMloki, minFeeMloki, proportional uint64) (uint64, error) {
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
