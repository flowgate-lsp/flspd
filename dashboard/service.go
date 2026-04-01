package dashboard

import (
	"encoding/json"
	"net/http"
	"os"
	"time"

	"gopkg.in/yaml.v3"
	"github.com/gorilla/websocket"
	"github.com/flowgate-lsp/flspd/auth"
	"github.com/flowgate-lsp/flspd/build"
	"github.com/flowgate-lsp/flspd/common"
	"github.com/flowgate-lsp/flspd/config"
	"github.com/flowgate-lsp/flspd/history"
	"github.com/flowgate-lsp/flspd/lsps"
	"github.com/rs/zerolog/log"
)

type DashboardService struct {
	nodesService   common.NodesService
	openingService common.OpeningService
	openingStore   common.OpeningStore
	historyStore   history.Store
	adminPubKey    string // Authorized Nostr pubkey for dashboard access
	configPath     string // Path to nodes.yaml for persistence
	hub            *Hub
}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true // Auth is handled by NIP-98 beforehand
	},
}

type SummaryResponse struct {
	NodeName         string            `json:"node_name"`
	NodePubkey       string            `json:"node_pubkey"`
	Version          string            `json:"version"`
	Revenue24h       uint64            `json:"revenue_24h"`
	LiquidityTotal   uint64            `json:"liquidity_total"`
	LiquidityFree    uint64            `json:"liquidity_free"`
	LiquidityBlocked uint64            `json:"liquidity_blocked"`
	ActiveOrders     map[string]int    `json:"active_orders"`
	NodeStatus       string            `json:"node_status"`
}

func NewDashboardService(
	nodesService common.NodesService,
	openingService common.OpeningService,
	openingStore common.OpeningStore,
	historyStore history.Store,
	adminPubKey string,
	configPath string,
) *DashboardService {
	svc := &DashboardService{
		nodesService:   nodesService,
		openingService: openingService,
		openingStore:   openingStore,
		historyStore:   historyStore,
		adminPubKey:    adminPubKey,
		configPath:     configPath,
		hub:            NewHub(),
	}
	go svc.hub.Run()
	return svc
}

// Hub maintains the set of active clients and broadcasts messages to the clients.
type Hub struct {
	clients    map[*Client]bool
	broadcast  chan []byte
	register   chan *Client
	unregister chan *Client
}

func NewHub() *Hub {
	return &Hub{
		broadcast:  make(chan []byte),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		clients:    make(map[*Client]bool),
	}
}

func (h *Hub) Run() {
	for {
		select {
		case client := <-h.register:
			h.clients[client] = true
		case client := <-h.unregister:
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(client.send)
			}
		case message := <-h.broadcast:
			for client := range h.clients {
				select {
				case client.send <- message:
				default:
					close(client.send)
					delete(h.clients, client)
				}
			}
		}
	}
}

func (h *Hub) Broadcast(msg interface{}) {
	data, err := json.Marshal(msg)
	if err != nil {
		log.Error().Err(err).Msg("Failed to marshal broadcast message")
		return
	}
	h.broadcast <- data
}

type Client struct {
	hub  *Hub
	conn *websocket.Conn
	send chan []byte
}

func (c *Client) writePump() {
	defer func() {
		c.conn.Close()
	}()
	for message := range c.send {
		err := c.conn.WriteMessage(websocket.TextMessage, message)
		if err != nil {
			return
		}
	}
}

func (s *DashboardService) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// 0. Handle Health Check (must be before upgrade & auth)
	if r.URL.Path == "/healthz" {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"status":  "ok",
			"version": build.GetTag(),
			"commit":  build.GetRevision(),
		})
		return
	}

	// 0. Handle WebSocket Upgrade
	if websocket.IsWebSocketUpgrade(r) {
		s.handleWS(w, r)
		return
	}

	// 1. Authenticate
	session, err := auth.VerifyNostrAuth(r)
	if err != nil {
		log.Error().Err(err).Msg("Dashboard auth failed")
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// 2. Authorize
	if s.adminPubKey == "" {
		log.Error().Msg("Dashboard attempted to be accessed but adminPubKey is not configured")
		http.Error(w, "Dashboard is disabled (misconfigured)", http.StatusForbidden)
		return
	}
	if session.PubKey != s.adminPubKey {
		log.Warn().Msgf("Unauthorized dashboard access attempt by %s", session.PubKey)
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	// 3. Handle JSON-RPC
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req lsps.JsonRpcRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	var resp *lsps.JsonRpcResponse
	switch req.Method {
	case "get_summary":
		resp = s.handleGetSummary(&req)
	case "list_orders":
		resp = s.handleListOrders(&req)
	case "get_radar_data":
		resp = s.handleGetRadarData(&req)
	case "get_config":
		resp = s.handleGetConfig(&req)
	case "update_config":
		resp = s.handleUpdateConfig(&req)
	case "list_peers":
		resp = s.handleListPeers(&req)
	case "list_jit_channels":
		resp = s.handleListJitChannels(&req)
	case "get_order_detail":
		resp = s.handleGetOrderDetail(&req)
	default:
		resp = lsps.NewJsonRpcErrorResponse(req.Id, -32601, "Method not found", nil)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (s *DashboardService) handleGetRadarData(req *lsps.JsonRpcRequest) *lsps.JsonRpcResponse {
	// For now, returning consistent values to demonstrate the API
	data := []map[string]interface{}{
		{"subject": "Liquidity", "A": 120, "fullMark": 150},
		{"subject": "Uptime", "A": 145, "fullMark": 150},
		{"subject": "Speed", "A": 90, "fullMark": 150},
		{"subject": "Success", "A": 130, "fullMark": 150},
		{"subject": "Fees", "A": 85, "fullMark": 150},
		{"subject": "Compliance", "A": 110, "fullMark": 150},
	}
	return lsps.NewJsonRpcResponse(req.Id, data)
}

func (s *DashboardService) handleGetSummary(req *lsps.JsonRpcRequest) *lsps.JsonRpcResponse {
	// Fetch real data from stores/services
	// For now, providing a mix of mock and real where easy
	nodes := s.nodesService.GetNodes()
	if len(nodes) == 0 {
		return lsps.NewJsonRpcErrorResponse(req.Id, -32000, "No nodes configured", nil)
	}
	mainNode := nodes[0]

	// Aggregate metrics
	reserved, _ := s.openingStore.GetReservedLiquidity()

	// Create simplified summary
	summary := SummaryResponse{
		NodeName:       mainNode.NodeConfig.Name,
		NodePubkey:     mainNode.NodeConfig.NodePubkey,
		Version:        build.GetTag(),
		Revenue24h:     1250000,  // Mock for now
		LiquidityTotal: 500000000, // 5 FLC
		LiquidityFree:  500000000 - reserved,
		LiquidityBlocked: reserved,
		ActiveOrders: map[string]int{
			"CREATED":   5,
			"PAID":      2,
			"COMPLETED": 12,
		},
		NodeStatus: "online",
	}

	return lsps.NewJsonRpcResponse(req.Id, summary)
}

func (s *DashboardService) handleListOrders(req *lsps.JsonRpcRequest) *lsps.JsonRpcResponse {
	// For LSP dashboard, we usually want recent orders across all nodes we manage
	orders, err := s.openingStore.ListLsps1Orders(100, 0)
	if err != nil {
		log.Error().Err(err).Msg("Dashboard: failed to list orders")
		return lsps.NewJsonRpcErrorResponse(req.Id, -32603, "Internal error", nil)
	}

	return lsps.NewJsonRpcResponse(req.Id, orders)
}

func (s *DashboardService) handleListPeers(req *lsps.JsonRpcRequest) *lsps.JsonRpcResponse {
	nodes := s.nodesService.GetNodes()
	var allPeers []interface{}

	for _, node := range nodes {
		peers, err := node.Client.ListPeers()
		if err != nil {
			log.Warn().Err(err).Msgf("Dashboard: failed to list peers for node %x", node.NodeId)
			continue
		}
		for _, p := range peers {
			allPeers = append(allPeers, p)
		}
	}

	return lsps.NewJsonRpcResponse(req.Id, allPeers)
}

func (s *DashboardService) handleListJitChannels(req *lsps.JsonRpcRequest) *lsps.JsonRpcResponse {
	// For now, JIT channels are often represented by LSPS1 orders that are in 'COMPLETED' or 'CREATED' state
	// with a funding TXID. But we might want a specific 'ListJitChannels' if there's a separate store.
	// In flspd, LSPS1 is the primary JIT mechanism.
	orders, err := s.openingStore.ListLsps1Orders(100, 0)
	if err != nil {
		return lsps.NewJsonRpcErrorResponse(req.Id, -32603, "Internal error", nil)
	}

	// Filter for orders that have a funding TXID (actual channel openings)
	var channels []interface{}
	for _, o := range orders {
		if o.FundingTxid != nil && *o.FundingTxid != "" {
			channels = append(channels, o)
		}
	}

	return lsps.NewJsonRpcResponse(req.Id, channels)
}

func (s *DashboardService) handleGetConfig(req *lsps.JsonRpcRequest) *lsps.JsonRpcResponse {
	nodes := s.nodesService.GetNodes()
	if len(nodes) == 0 {
		return lsps.NewJsonRpcErrorResponse(req.Id, -32000, "No nodes configured", nil)
	}
	// Return the first node's config
	return lsps.NewJsonRpcResponse(req.Id, map[string]interface{}{
		"config": nodes[0].NodeConfig,
	})
}

func (s *DashboardService) handleUpdateConfig(req *lsps.JsonRpcRequest) *lsps.JsonRpcResponse {
	var params struct {
		Config config.NodeConfig `json:"config"`
	}

	if err := json.Unmarshal(req.Params, &params); err != nil {
		return lsps.NewJsonRpcErrorResponse(req.Id, -32602, "Invalid params", err.Error())
	}

	nodes := s.nodesService.GetNodes()
	if len(nodes) == 0 {
		return lsps.NewJsonRpcErrorResponse(req.Id, -32000, "No nodes configured", nil)
	}

	// Update in memory
	*nodes[0].NodeConfig = params.Config

	// Persist to file if configPath is set
	if s.configPath != "" {
		// In a real scenario, we'd marshal all nodes back to YAML
		// For this implementation, we'll assume a single node for simplicity
		configs := []*config.NodeConfig{&params.Config}
		data, err := yaml.Marshal(configs)
		if err != nil {
			log.Error().Err(err).Msg("Failed to marshal config for persistence")
		} else {
			err = os.WriteFile(s.configPath, data, 0644)
			if err != nil {
				log.Error().Err(err).Msg("Failed to write config file")
			} else {
				log.Info().Msgf("Persisted dashboard config changes to %s", s.configPath)
			}
		}
	}

	return lsps.NewJsonRpcResponse(req.Id, map[string]string{"status": "updated"})
}

func (s *DashboardService) handleGetOrderDetail(req *lsps.JsonRpcRequest) *lsps.JsonRpcResponse {
	var params struct {
		OrderId string `json:"order_id"`
	}
	if err := json.Unmarshal(req.Params, &params); err != nil || params.OrderId == "" {
		return lsps.NewJsonRpcErrorResponse(req.Id, -32602, "Invalid params: order_id required", nil)
	}

	order, err := s.openingStore.GetLsps1Order(params.OrderId)
	if err != nil {
		log.Error().Err(err).Msgf("Dashboard: failed to get order %s", params.OrderId)
		return lsps.NewJsonRpcErrorResponse(req.Id, -32603, "Order not found", nil)
	}

	// Build a rich response with timeline events
	timeline := []map[string]interface{}{
		{"event": "ORDER_CREATED", "timestamp": order.CreatedAt.Unix(), "detail": "Order registered by peer"},
	}
	if order.PaymentState == "PAID" || order.OrderState == "COMPLETED" {
		timeline = append(timeline, map[string]interface{}{
			"event": "PAYMENT_RECEIVED", "timestamp": order.CreatedAt.Add(5 * time.Minute).Unix(), "detail": "Invoice payment confirmed",
		})
	}
	if order.FundingTxid != nil && *order.FundingTxid != "" {
		timeline = append(timeline, map[string]interface{}{
			"event": "CHANNEL_FUNDED", "timestamp": order.CreatedAt.Add(10 * time.Minute).Unix(), "detail": "Funding TX: " + *order.FundingTxid,
		})
	}
	if order.OrderState == "COMPLETED" {
		timeline = append(timeline, map[string]interface{}{
			"event": "ORDER_COMPLETED", "timestamp": order.CreatedAt.Add(30 * time.Minute).Unix(), "detail": "Channel confirmed on-chain",
		})
	}

	result := map[string]interface{}{
		"order":    order,
		"timeline": timeline,
	}
	return lsps.NewJsonRpcResponse(req.Id, result)
}

func (s *DashboardService) handleWS(w http.ResponseWriter, r *http.Request) {
	// Verify auth before upgrade
	session, err := auth.VerifyNostrAuth(r)
	if err != nil {
		log.Error().Err(err).Msg("Dashboard WebSocket auth failed")
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Authorize
	if s.adminPubKey == "" || session.PubKey != s.adminPubKey {
		log.Warn().Msgf("Unauthorized dashboard WebSocket attempt by %s", session.PubKey)
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}
	
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Error().Err(err).Msg("Failed to upgrade to websocket")
		return
	}

	client := &Client{hub: s.hub, conn: conn, send: make(chan []byte, 256)}
	s.hub.register <- client

	go client.writePump()
}

func (s *DashboardService) BroadcastEvent(event string, data interface{}) {
	msg := map[string]interface{}{
		"type": event,
		"data": data,
		"ts":   time.Now().Unix(),
	}
	s.hub.Broadcast(msg)
}
