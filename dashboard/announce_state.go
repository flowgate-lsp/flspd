package dashboard

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"time"

	"github.com/flowgate-lsp/flspd/core"
	"github.com/flowgate-lsp/flspd/lsps"
	"github.com/rs/zerolog/log"
)

// AnnounceStateResponse is the answer of get_announce_state and
// set_announce_state: the node's NIP-LSP announcement status (T8-2).
type AnnounceStateResponse struct {
	NodePubkey string   `json:"node_pubkey"`
	Enabled    bool     `json:"enabled"`
	Status     string   `json:"status"`
	UpdatedAt  *string  `json:"updated_at"` // RFC 3339; null while never changed
	Relays     []string `json:"relays"`
}

func (s *Handlers) announceStateResponse(node *core.Node, st *core.AnnounceState) AnnounceStateResponse {
	resp := AnnounceStateResponse{
		NodePubkey: hex.EncodeToString(node.NodeId),
		Enabled:    node.NodeConfig.Load().Announcing(),
		Status:     string(st.Status),
		Relays:     s.announceRelays,
	}
	if resp.Relays == nil {
		resp.Relays = []string{}
	}
	if !st.UpdatedAt.IsZero() {
		t := st.UpdatedAt.UTC().Format(time.RFC3339)
		resp.UpdatedAt = &t
	}
	return resp
}

// announceNode returns the node the dashboard edits (the first one, as
// get_config and update_config assume), or an error response.
func (s *Handlers) announceNode(req *lsps.JsonRpcRequest) (*core.Node, *lsps.JsonRpcResponse) {
	if s.announceState == nil {
		return nil, lsps.NewJsonRpcErrorResponse(req.Id, -32000, "Announcement state unavailable", nil)
	}
	nodes := s.nodesService.GetNodes()
	if len(nodes) == 0 {
		return nil, lsps.NewJsonRpcErrorResponse(req.Id, -32000, "No nodes configured", nil)
	}
	return nodes[0], nil
}

func (s *Handlers) handleGetAnnounceState(req *lsps.JsonRpcRequest) *lsps.JsonRpcResponse {
	node, errResp := s.announceNode(req)
	if errResp != nil {
		return errResp
	}
	st, err := s.announceState.GetAnnounceState(context.Background(), node.NodeId)
	if err != nil {
		log.Error().Err(err).Msg("Dashboard: failed to read announcement state")
		return lsps.NewJsonRpcErrorResponse(req.Id, -32603, "Internal error", nil)
	}
	return lsps.NewJsonRpcResponse(req.Id, s.announceStateResponse(node, st))
}

// handleSetAnnounceState applies a dashboard action: pause, resume or
// unlist (core.NextAnnounceStatus).
func (s *Handlers) handleSetAnnounceState(req *lsps.JsonRpcRequest) *lsps.JsonRpcResponse {
	var params struct {
		Action string `json:"action"`
	}
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return lsps.NewJsonRpcErrorResponse(req.Id, -32602, "Invalid params", err.Error())
	}
	action := core.AnnounceAction(params.Action)
	switch action {
	case core.AnnouncePause, core.AnnounceResume, core.AnnounceUnlist:
	default:
		return lsps.NewJsonRpcErrorResponse(req.Id, -32602, "Invalid params: action must be pause, resume or unlist", nil)
	}

	node, errResp := s.announceNode(req)
	if errResp != nil {
		return errResp
	}
	st, err := s.announceState.ApplyAnnounceAction(context.Background(), node.NodeId, action)
	if errors.Is(err, core.ErrAnnounceTransition) {
		return lsps.NewJsonRpcErrorResponse(req.Id, -32602, "Action not allowed", err.Error())
	}
	if err != nil {
		log.Error().Err(err).Msg("Dashboard: failed to apply announcement action")
		return lsps.NewJsonRpcErrorResponse(req.Id, -32603, "Internal error", nil)
	}
	log.Info().Msgf("Dashboard: announcement %s -> %s", action, st.Status)
	if s.onAnnounceStateChange != nil {
		s.onAnnounceStateChange(st)
	}
	return lsps.NewJsonRpcResponse(req.Id, s.announceStateResponse(node, st))
}
