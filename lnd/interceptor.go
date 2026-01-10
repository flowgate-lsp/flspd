package lnd

import (
	"context"
	"sync"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/flokiorg/flnd/lnrpc"
	"github.com/flokiorg/flnd/lnrpc/routerrpc"
	"github.com/flowgate-lsp/flspd/common"
	"github.com/flowgate-lsp/flspd/config"
	"github.com/flowgate-lsp/flspd/lightning"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type LndHtlcInterceptor struct {
	interceptor   common.InterceptHandler
	config        *config.NodeConfig
	client        *LndClient
	stopRequested bool
	initWg        sync.WaitGroup
	doneWg        sync.WaitGroup
	ctx           context.Context
	cancel        context.CancelFunc
}

func NewLndHtlcInterceptor(
	conf *config.NodeConfig,
	client *LndClient,
	interceptor common.InterceptHandler,
) (*LndHtlcInterceptor, error) {
	i := &LndHtlcInterceptor{
		config:      conf,
		client:      client,
		interceptor: interceptor,
	}

	i.initWg.Add(1)

	return i, nil
}

func (i *LndHtlcInterceptor) Start() error {
	ctx, cancel := context.WithCancel(context.Background())
	i.ctx = ctx
	i.cancel = cancel
	i.stopRequested = false

	return i.intercept()
}

func (i *LndHtlcInterceptor) Stop() error {
	// Setting stopRequested to true will make the interceptor stop receiving.
	i.stopRequested = true

	// Wait until all already received htlcs are handled, responses sent back.
	i.doneWg.Wait()

	// Close the grpc connection.
	i.cancel()
	return nil
}

func (i *LndHtlcInterceptor) WaitStarted() {
	i.initWg.Wait()
}

func (i *LndHtlcInterceptor) intercept() error {
	inited := false
	defer func() {
		if !inited {
			i.initWg.Done()
		}
		log.Info().Msgf("LND intercept(): stopping. Waiting for in-progress interceptions to complete.")
		i.doneWg.Wait()
	}()

	for {
		if i.ctx.Err() != nil {
			return i.ctx.Err()
		}

		log.Info().Msgf("Connecting LND HTLC interceptor.")
		interceptorClient, err := i.client.routerClient.HtlcInterceptor(i.ctx)
		if err != nil {
			log.Info().Msgf("routerClient.HtlcInterceptor(): %v", err)
			<-time.After(time.Second)
			continue
		}

		for {
			if i.ctx.Err() != nil {
				return i.ctx.Err()
			}

			if !inited {
				inited = true
				i.initWg.Done()
			}

			// Stop receiving if stop if requested. The defer func on top of this
			// function will assure all htlcs that are currently being processed
			// will complete.
			if i.stopRequested {
				return nil
			}

			request, err := interceptorClient.Recv()
			if err != nil {
				// If it is  just the error result of the context cancellation
				// the we exit silently.
				status, ok := status.FromError(err)
				if ok && status.Code() == codes.Canceled {
					log.Info().Msgf("Got code canceled. Break.")
					break
				}

				// Otherwise it an unexpected error, we fail the test.
				log.Info().Msgf("unexpected error in interceptor.Recv() %v", err)
				break
			}

			i.doneWg.Add(1)
			go func() {
				scid := lightning.ShortChannelID(request.OutgoingRequestedChanId)
				interceptResult := i.interceptor.Intercept(common.InterceptRequest{
					Identifier:         request.IncomingCircuitKey.String(),
					Scid:               scid,
					PaymentHash:        request.PaymentHash,
					IncomingAmountMsat: request.IncomingAmountMsat,
					OutgoingAmountMsat: request.OutgoingAmountMsat,
					IncomingExpiry:     request.IncomingExpiry,
					OutgoingExpiry:     request.OutgoingExpiry,
				})
				switch interceptResult.Action {
				case common.INTERCEPT_RESUME_WITH_ONION:
					interceptorClient.Send(i.createOnionResponse(interceptResult, request))
				case common.INTERCEPT_FAIL_HTLC_WITH_CODE:
					log.Info().Msgf("paymenthash %x, failing htlc with code '%x'", request.PaymentHash, interceptResult.FailureCode)
					interceptorClient.Send(&routerrpc.ForwardHtlcInterceptResponse{
						IncomingCircuitKey: request.IncomingCircuitKey,
						Action:             routerrpc.ResolveHoldForwardAction_FAIL,
						FailureCode:        i.mapFailureCode(interceptResult.FailureCode),
					})
				case common.INTERCEPT_RESUME:
					fallthrough
				default:
					interceptorClient.Send(&routerrpc.ForwardHtlcInterceptResponse{
						IncomingCircuitKey: request.IncomingCircuitKey,
						Action:             routerrpc.ResolveHoldForwardAction_RESUME,
					})
				}

				i.doneWg.Done()
			}()
		}

		<-time.After(time.Second)
	}
}

func (i *LndHtlcInterceptor) mapFailureCode(original common.InterceptFailureCode) lnrpc.Failure_FailureCode {
	switch original {
	case common.FAILURE_TEMPORARY_CHANNEL_FAILURE:
		return lnrpc.Failure_TEMPORARY_CHANNEL_FAILURE
	case common.FAILURE_TEMPORARY_NODE_FAILURE:
		return lnrpc.Failure_TEMPORARY_NODE_FAILURE
	case common.FAILURE_INCORRECT_OR_UNKNOWN_PAYMENT_DETAILS:
		return lnrpc.Failure_INCORRECT_OR_UNKNOWN_PAYMENT_DETAILS
	default:
		log.Info().Msgf("Unknown failure code %v, default to temporary channel failure.", original)
		return lnrpc.Failure_TEMPORARY_CHANNEL_FAILURE
	}
}

func (i *LndHtlcInterceptor) createOnionResponse(interceptResult common.InterceptResult, request *routerrpc.ForwardHtlcInterceptRequest) *routerrpc.ForwardHtlcInterceptResponse {
	return &routerrpc.ForwardHtlcInterceptResponse{
		IncomingCircuitKey: request.IncomingCircuitKey,
		Action:             routerrpc.ResolveHoldForwardAction_RESUME_MODIFIED,
		OutAmountMsat:      interceptResult.AmountMsat,
	}
}
