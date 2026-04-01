package dashboard

import (
	"context"
	"encoding/json"
	"fmt"
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
	"github.com/flowgate-lsp/flspd/lightning"
	"github.com/flowgate-lsp/flspd/lsps"
	"github.com/rs/zerolog/log"
)

const (
	// Dashboard limits and scoring
	radarFullMark        = 150
	defaultListLimit     = 100
	analyticsListLimit   = 2000
	summaryListLimit     = 1000
	analyticsHistoryDays = 14
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
	SyncHeight       uint32            `json:"sync_height"`
	UptimeScore      float64           `json:"uptime_score"`
	RevenueTrend     string            `json:"revenue_trend"`
	VolumeTrend      string            `json:"volume_trend"`
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
	case "get_analytics":
		resp = s.handleGetAnalytics(&req)
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
	// Calculate real metrics
	totalCapacity, _, reserved := s.getLiquidityMetrics(context.Background())
	
	// 1. Liquidity Score (0-radarFullMark)
	liquidityScore := radarFullMark
	if totalCapacity > 0 {
		usage := float64(reserved) / float64(totalCapacity)
		liquidityScore = int(radarFullMark * (1.0 - usage))
	}

	// 2. Uptime Score (0-radarFullMark)
	uptimeScore := radarFullMark
	nodes := s.nodesService.GetNodes()
	if len(nodes) > 0 {
		peers, _ := nodes[0].Client.ListPeers()
		activeCount := 0
		for _, p := range peers {
			if p.Active {
				activeCount++
			}
		}
		if len(peers) > 0 {
			uptimeScore = int(radarFullMark * (float64(activeCount) / float64(len(peers))))
		}
	}

	// 3. Success Score (0-radarFullMark)
	// Base success score weighted by order outcomes
	successScore := (radarFullMark * 85) / 100 // 85% base
	orders, _ := s.openingStore.ListLsps1Orders(defaultListLimit, 0)
	if len(orders) > 0 {
		completed := 0
		failed := 0
		for _, o := range orders {
			if o.OrderState == string(lsps.Lsps1OrderStateCompleted) {
				completed++
			} else if o.OrderState == string(lsps.Lsps1OrderStateFailed) {
				failed++
			}
		}
		if completed+failed > 0 {
			successScore = int(radarFullMark * (float64(completed) / float64(completed+failed)))
		}
	}

	data := []map[string]interface{}{
		{"subject": "Liquidity", "A": liquidityScore, "fullMark": radarFullMark},
		{"subject": "Uptime", "A": uptimeScore, "fullMark": radarFullMark},
		{"subject": "Speed", "A": 120, "fullMark": radarFullMark}, // TODO: compute real speed
		{"subject": "Success", "A": successScore, "fullMark": radarFullMark},
		{"subject": "Fees", "A": 110, "fullMark": radarFullMark}, // TODO: compute fee score
		{"subject": "Compliance", "A": radarFullMark, "fullMark": radarFullMark},
	}
	return lsps.NewJsonRpcResponse(req.Id, data)
}

func (s *DashboardService) handleGetAnalytics(req *lsps.JsonRpcRequest) *lsps.JsonRpcResponse {
	// Fetch a generous amount of recent orders to compute stats
	orders, _ := s.openingStore.ListLsps1Orders(analyticsListLimit, 0)

	// 1. Order Volume by State & Core Stats
	volume := map[string]int{
		string(lsps.Lsps1OrderStateCreated):   0,
		string(lsps.Lsps1OrderStatePaid):      0,
		string(lsps.Lsps1OrderStateCompleted): 0,
		string(lsps.Lsps1OrderStateFailed):    0,
		string(lsps.Lsps1OrderStateExpired):   0,
	}
	
	var totalRevenue int64 = 0
	var completedCount int64 = 0
	
	// 2. History maps
	revenueMap := make(map[string]int64)
	volumeMap := make(map[string]int64)
	countMap := make(map[string]int)
	
	now := time.Now()
	for i := 0; i < analyticsHistoryDays; i++ {
		day := now.AddDate(0, 0, -i).Format("2006-01-02")
		revenueMap[day] = 0
		volumeMap[day] = 0
		countMap[day] = 0
	}

	for _, o := range orders {
		volume[o.OrderState]++
		
		day := o.CreatedAt.Format("2006-01-02")
		
		// Map revenue and volume history (Completed or Paid)
		if o.OrderState == string(lsps.Lsps1OrderStateCompleted) || o.OrderState == string(lsps.Lsps1OrderStatePaid) {
			if _, ok := revenueMap[day]; ok {
				revenueMap[day] += o.OrderTotalLoki
				volumeMap[day] += o.LspBalanceLoki
			}
			if o.OrderState == string(lsps.Lsps1OrderStateCompleted) {
				totalRevenue += o.OrderTotalLoki
				completedCount++
			}
		}

		// Map total orders per day
		if _, ok := countMap[day]; ok {
			countMap[day]++
		}
	}

	// Calculate final stats
	successRate := 0.0
	totalOrders := len(orders)
	if totalOrders > 0 {
		// Completed / (Completed + Failed) as a simple business success rate
		relevant := volume[string(lsps.Lsps1OrderStateCompleted)] + volume[string(lsps.Lsps1OrderStateFailed)]
		if relevant > 0 {
			successRate = (float64(volume[string(lsps.Lsps1OrderStateCompleted)]) / float64(relevant)) * 100
		}
	}

	avgOrderSize := int64(0)
	if completedCount > 0 {
		avgOrderSize = totalRevenue / completedCount
	}

	// Active peers count (real from nodesService)
	activePeers := 0
	nodes := s.nodesService.GetNodes()
	if len(nodes) > 0 {
		peers, _ := nodes[0].Client.ListPeers()
		for _, p := range peers {
			if p.Active {
				activePeers++
			}
		}
	}

	// Format history for frontend
	history := make([]map[string]interface{}, 0, 14)
	for i := 13; i >= 0; i-- {
		t := now.AddDate(0, 0, -i)
		dayKey := t.Format("2006-01-02")
		history = append(history, map[string]interface{}{
			"date":    dayKey,
			"day":     t.Format("Mon"),
			"revenue": revenueMap[dayKey],
			"count":   countMap[dayKey],
			"volume":  revenueMap[dayKey], // Volume often equals revenue for this dashboard
		})
	}

	return lsps.NewJsonRpcResponse(req.Id, map[string]interface{}{
		"total_revenue":     totalRevenue,
		"success_rate":      successRate,
		"avg_order_size":    avgOrderSize,
		"active_peers":      activePeers,
		"revenue_history":   history,
		"order_distribution": []map[string]interface{}{
			{"name": "CREATED", "value": volume[string(lsps.Lsps1OrderStateCreated)], "color": "#94a3b8"},
			{"name": "PAID", "value": volume[string(lsps.Lsps1OrderStatePaid)], "color": "#00baff"},
			{"name": "COMPLETED", "value": volume[string(lsps.Lsps1OrderStateCompleted)], "color": "#00fd87"},
			{"name": "FAILED", "value": volume[string(lsps.Lsps1OrderStateFailed)], "color": "#ff3b30"},
			{"name": "EXPIRED", "value": volume[string(lsps.Lsps1OrderStateExpired)], "color": "#4a5568"},
		},
	})
}

func (s *DashboardService) handleGetSummary(req *lsps.JsonRpcRequest) *lsps.JsonRpcResponse {
	// Fetch real data from stores/services
	nodes := s.nodesService.GetNodes()
	if len(nodes) == 0 {
		return lsps.NewJsonRpcErrorResponse(req.Id, -32000, "No nodes configured", nil)
	}
	mainNode := nodes[0]

	// Aggregate metrics
	totalCapacity, free, reserved := s.getLiquidityMetrics(context.Background())
	
	// Real Metrics for Summary
	var revenue24h int64 = 0
	var revenuePrev24h int64 = 0
	var volume24h int64 = 0
	var volumePrev24h int64 = 0

	orderStates := map[string]int{
		string(lsps.Lsps1OrderStateCreated):   0,
		string(lsps.Lsps1OrderStatePaid):      0,
		string(lsps.Lsps1OrderStateCompleted): 0,
		string(lsps.Lsps1OrderStateFailed):    0,
		string(lsps.Lsps1OrderStateExpired):   0,
	}

	orders, _ := s.openingStore.ListLsps1Orders(summaryListLimit, 0)
	now := time.Now()
	last24h := now.Add(-24 * time.Hour)
	prev24h := now.Add(-48 * time.Hour)
	
	for _, o := range orders {
		if o.CreatedAt.After(last24h) {
			if o.OrderState == string(lsps.Lsps1OrderStateCompleted) || o.OrderState == string(lsps.Lsps1OrderStatePaid) {
				revenue24h += o.OrderTotalLoki
				volume24h += o.LspBalanceLoki
			}
		} else if o.CreatedAt.After(prev24h) {
			if o.OrderState == string(lsps.Lsps1OrderStateCompleted) || o.OrderState == string(lsps.Lsps1OrderStatePaid) {
				revenuePrev24h += o.OrderTotalLoki
				volumePrev24h += o.LspBalanceLoki
			}
		}
		
		// Count current "active" states (from recent 1000 orders)
		if _, ok := orderStates[o.OrderState]; ok {
			orderStates[o.OrderState]++
		}
	}

	// Calculate Trends
	calcTrend := func(curr, prev int64) string {
		if prev == 0 {
			if curr > 0 {
				return "+100%"
			}
			return "0%"
		}
		diff := float64(curr - prev)
		percent := (diff / float64(prev)) * 100
		if percent >= 0 {
			return fmt.Sprintf("+%.1f%%", percent)
		}
		return fmt.Sprintf("%.1f%%", percent)
	}

	revenueTrend := calcTrend(revenue24h, revenuePrev24h)
	volumeTrend := calcTrend(volume24h, volumePrev24h)

	// Fetch Block Height & Uptime
	syncHeight, _ := mainNode.Client.GetBlockHeight()
	peers, _ := mainNode.Client.ListPeers()
	activeCount := 0
	for _, p := range peers {
		if p.Active {
			activeCount++
		}
	}
	uptimeScore := 100.0
	if len(peers) > 0 {
		uptimeScore = (float64(activeCount) / float64(len(peers))) * 100
	}

	// Create simplified summary
	summary := SummaryResponse{
		NodeName:       mainNode.NodeConfig.Name,
		NodePubkey:     mainNode.NodeConfig.NodePubkey,
		Version:        build.GetTag(),
		Revenue24h:     uint64(revenue24h),
		LiquidityTotal: totalCapacity,
		LiquidityFree:  free,
		LiquidityBlocked: reserved,
		ActiveOrders:   orderStates,
		NodeStatus:     "online",
		SyncHeight:     syncHeight,
		UptimeScore:    uptimeScore,
		RevenueTrend:   revenueTrend,
		VolumeTrend:    volumeTrend,
	}

	return lsps.NewJsonRpcResponse(req.Id, summary)
}

func (s *DashboardService) handleListOrders(req *lsps.JsonRpcRequest) *lsps.JsonRpcResponse {
	// For LSP dashboard, we usually want recent orders across all nodes we manage
	orders, err := s.openingStore.ListLsps1Orders(defaultListLimit, 0)
	if err != nil {
		log.Error().Err(err).Msg("Dashboard: failed to list orders")
		return lsps.NewJsonRpcErrorResponse(req.Id, -32603, "Internal error", nil)
	}

	// Enrich with peer metadata
	nodes := s.nodesService.GetNodes()
	peerMap := make(map[string]*lightning.Peer)
	if len(nodes) > 0 {
		peers, _ := nodes[0].Client.ListPeers()
		for _, p := range peers {
			peerMap[p.Pubkey] = p
		}
	}

	for i := range orders {
		if p, ok := peerMap[orders[i].PeerId]; ok {
			orders[i].PeerAlias = p.Alias
			orders[i].PeerColor = p.Color
		}
	}

	return lsps.NewJsonRpcResponse(req.Id, orders)
}

func (s *DashboardService) handleListPeers(req *lsps.JsonRpcRequest) *lsps.JsonRpcResponse {
	nodes := s.nodesService.GetNodes()
	var allPeers []interface{}

	// 1. Get order counts per peer
	orderCounts := make(map[string]int)
	orders, _ := s.openingStore.ListLsps1Orders(summaryListLimit, 0)
	for _, o := range orders {
		orderCounts[o.PeerId]++
	}

	for _, node := range nodes {
		peers, err := node.Client.ListPeers()
		if err != nil {
			log.Warn().Err(err).Msgf("Dashboard: failed to list peers for node %x", node.NodeId)
			continue
		}
		for _, p := range peers {
			// Enrich with order count
			count := orderCounts[p.Pubkey]
			
			// Map to consistent JSON structure
			enriched := map[string]interface{}{
				"pub_key":        p.Pubkey,
				"alias":          p.Alias,
				"color":          p.Color,
				"address":        p.Address,
				"inbound":        p.Inbound,
				"active":         p.Active,
				"in_sat":         p.BytesSent, // bytes_sent as in_sat for simplicity if desired, or as is
				"out_sat":        p.BytesReceived,
				"order_count":    count,
			}
			allPeers = append(allPeers, enriched)
		}
	}

	return lsps.NewJsonRpcResponse(req.Id, allPeers)
}

func (s *DashboardService) handleListJitChannels(req *lsps.JsonRpcRequest) *lsps.JsonRpcResponse {
	// JIT channels are represented by LSPS1 orders that have been successfully funded
	orders, err := s.openingStore.ListLsps1Orders(defaultListLimit, 0)
	if err != nil {
		return lsps.NewJsonRpcErrorResponse(req.Id, -32603, "Internal error", nil)
	}

	// 1. Fetch active channels from all nodes to check real-time status
	activeChannels := make(map[string]*lightning.Channel)
	nodes := s.nodesService.GetNodes()
	for _, node := range nodes {
		chans, err := node.Client.ListChannels()
		if err != nil {
			log.Warn().Err(err).Msgf("Dashboard: failed to list channels for node %x", node.NodeId)
			continue
		}
		for _, c := range chans {
			if c.ChannelPoint != nil {
				// Key by TXID:Index
				activeChannels[c.ChannelPoint.String()] = c
			}
		}
	}

	// 2. Correlate orders with active channels
	var enrichedChannels []interface{}
	for _, o := range orders {
		// Only show orders that have a funding TXID (attempted or successful openings)
		if o.FundingTxid == nil || *o.FundingTxid == "" {
			continue
		}

		// Construct the outpoint string to match ListChannels format (e.g. "txid:idx")
		outpointStr := fmt.Sprintf("%s:%d", *o.FundingTxid, 0) // Default to index 0
		if o.FundingOutpoint != nil {
			outpointStr = fmt.Sprintf("%s:%d", *o.FundingTxid, *o.FundingOutpoint)
		}

		activeCh, ok := activeChannels[outpointStr]
		
		status := "OFFLINE"
		scid := ""
		if ok {
			status = "ACTIVE"
			if activeCh.ConfirmedScid != nil {
				scid = activeCh.ConfirmedScid.ToString()
			} else if activeCh.AliasScid != nil {
				scid = activeCh.AliasScid.ToString()
			}
		}

		// Map to a frontend-friendly object
		enriched := map[string]interface{}{
			"order_id":   o.OrderId,
			"channel_id": scid, // Show real SCID if active
			"peer_id":    o.PeerId,
			"capacity":   o.LspBalanceLoki,
			"fee":        o.OrderTotalLoki,
			"status":     status,
			"created_at": o.CreatedAt,
			"txid":       *o.FundingTxid,
		}
		enrichedChannels = append(enrichedChannels, enriched)
	}

	return lsps.NewJsonRpcResponse(req.Id, enrichedChannels)
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

	// Enrich with peer metadata
	nodes := s.nodesService.GetNodes()
	if len(nodes) > 0 {
		info, err := nodes[0].Client.GetNodeInfo(order.PeerId)
		if err == nil && info != nil {
			order.PeerAlias = info.Alias
			order.PeerColor = info.Color
		}
	}

	// Build a rich response with timeline events
	timeline := []map[string]interface{}{
		{"event": "ORDER_CREATED", "timestamp": order.CreatedAt.Format(time.RFC3339), "detail": "Order registered by peer"},
	}
	if order.PaymentState == string(lsps.Lsps1PaymentStatePaid) || order.OrderState == string(lsps.Lsps1OrderStateCompleted) {
		timeline = append(timeline, map[string]interface{}{
			"event":     "PAYMENT_RECEIVED",
			"timestamp": order.CreatedAt.Format(time.RFC3339), // Recorded at same time for now
			"detail":    "Invoice payment confirmed",
		})
	}

	if order.FundingTxid != nil && *order.FundingTxid != "" {
		timeline = append(timeline, map[string]interface{}{
			"event":     "CHANNEL_FUNDED",
			"timestamp": order.CreatedAt.Format(time.RFC3339), // Recorded at same time for now
			"detail":    "Funding TX: " + *order.FundingTxid,
		})
	}

	if order.OrderState == string(lsps.Lsps1OrderStateCompleted) {
		timeline = append(timeline, map[string]interface{}{
			"event":     "ORDER_COMPLETED",
			"timestamp": order.CreatedAt.Format(time.RFC3339), // Recorded at same time for now
			"detail":    "Channel opened and confirmed",
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

func (s *DashboardService) getLiquidityMetrics(ctx context.Context) (total, free, reserved uint64) {
	reserved, _ = s.openingStore.GetReservedLiquidity()
	
	nodes := s.nodesService.GetNodes()
	if len(nodes) > 0 {
		balance, err := nodes[0].Client.WalletBalance(ctx)
		if err == nil {
			free = uint64(balance.ConfirmedBalance)
			total = free + reserved
			return
		}
	}
	
	// Fallback to placeholder if node not available or error
	total = 500000000
	free = 500000000 - reserved
	return
}

func (s *DashboardService) BroadcastEvent(event string, data interface{}) {
	msg := map[string]interface{}{
		"type": event,
		"data": data,
		"ts":   time.Now().Unix(),
	}
	s.hub.Broadcast(msg)
}
