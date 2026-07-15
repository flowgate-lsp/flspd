// Package dashboard is the admin dashboard's transport: NIP-98 HTTP auth,
// the admin-pubkey authorization gate, JSON-RPC-over-HTTP envelope
// marshal/unmarshal, and a websocket event feed for real-time push -- it
// owns none of the dashboard's business rules (config CRUD, analytics/
// liquidity aggregation) -- see dashboard (github.com/flowgate-lsp/flspd/
// dashboard) for that.
package dashboard

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/flowgate-lsp/flspd/lsps"
	"github.com/flowgate-lsp/flspd/platform/build"
	"github.com/flowgate-lsp/flspd/transport/auth"
	"github.com/gorilla/websocket"
	"github.com/rs/zerolog/log"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true // Auth is handled by NIP-98 beforehand
	},
}

// Router dispatches an already-decoded JSON-RPC request to whatever
// implements the dashboard's methods. *dashboard.Handlers is the only real
// implementation, but Server depends on this interface rather than that
// concrete type so the transport's own behavior (routing, auth, the
// websocket upgrade/event feed) can be tested without needing Handlers'
// business-logic dependencies.
type Router interface {
	Route(req *lsps.JsonRpcRequest) *lsps.JsonRpcResponse
}

// Server is the dashboard's http.Handler: NIP-98 auth, the admin-pubkey
// authorization gate, JSON-RPC dispatch via Router, and the websocket
// upgrade/event feed. It has no dashboard business logic of its own.
type Server struct {
	adminPubKey string // Authorized Nostr pubkey for dashboard access
	handlers    Router
	hub         *Hub
}

func NewServer(adminPubKey string, handlers Router) *Server {
	s := &Server{
		adminPubKey: adminPubKey,
		handlers:    handlers,
		hub:         NewHub(),
	}
	go s.hub.Run()
	return s
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

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
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

	resp := s.handlers.Route(&req)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (s *Server) handleWS(w http.ResponseWriter, r *http.Request) {
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

func (s *Server) BroadcastEvent(event string, data interface{}) {
	msg := map[string]interface{}{
		"type": event,
		"data": data,
		"ts":   time.Now().Unix(),
	}
	s.hub.Broadcast(msg)
}
