package lsps

import (
	"github.com/flowgate-lsp/flspd/common"
	"github.com/rs/zerolog/log"
)

// handleLsps0ListProtocols returns the list of supported protocols (2 for now, soon 1 and 5)
func (s *Server) handleLsps0ListProtocols(req *JsonRpcRequest) *JsonRpcResponse {
	log.Trace().
		Str("component", string(common.LogComponentLSPS0)).
		Str("action", string(common.LogActionRequest)).
		Msg("LSPS0: Received list_protocols request")
	// TODO: dynamically update this list as we implement more protocols
	result := map[string][]int{
		"protocols": {1, 2, 5},
	}
	log.Trace().
		Str("component", string(common.LogComponentLSPS0)).
		Str("action", string(common.LogActionResponse)).
		Msgf("LSPS0: list_protocols result: %+v", result)
	return NewJsonRpcResponse(req.Id, result)
}
