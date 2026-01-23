package common

import "github.com/rs/zerolog/log"

type CombinedHandler struct {
	handlers []InterceptHandler
}

func NewCombinedHandler(handlers ...InterceptHandler) *CombinedHandler {
	return &CombinedHandler{
		handlers: handlers,
	}
}

func (c *CombinedHandler) Intercept(req InterceptRequest) InterceptResult {
	for i, handler := range c.handlers {
		res := handler.Intercept(req)
		log.Debug().
			Str("component", string(LogComponentIntercept)).
			Str("req", req.String()).
			Int("handler_index", i).
			Str("result", res.String()).
			Msg("Intercept: Handler processed request")
		if res.Action != INTERCEPT_RESUME {
			return res
		}
	}

	return InterceptResult{
		Action: INTERCEPT_RESUME,
	}
}
