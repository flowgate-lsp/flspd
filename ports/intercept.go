package ports

import "github.com/flowgate-lsp/flspd/core"

type InterceptHandler interface {
	Intercept(req core.InterceptRequest) core.InterceptResult
}
