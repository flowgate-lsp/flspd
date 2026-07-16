package interceptor

import (
	"bytes"
	"context"
	"encoding/hex"
	"fmt"
	"math/big"
	"time"

	"github.com/flokiorg/go-flokicoin/wire"
	"github.com/flowgate-lsp/flspd/chain"
	"github.com/flowgate-lsp/flspd/common"
	"github.com/flowgate-lsp/flspd/core"
	"github.com/flowgate-lsp/flspd/history"
	"github.com/flowgate-lsp/flspd/lightning"
	"github.com/flowgate-lsp/flspd/notifications"
	"github.com/flowgate-lsp/flspd/platform/config"
	"github.com/flowgate-lsp/flspd/ports"
	"github.com/rs/zerolog/log"
	"golang.org/x/sync/singleflight"
)

type Interceptor struct {
	nodeid []byte
	client lightning.Client
	// config is an atomic ref, not a raw pointer, so it can be hot-swapped
	// by the dashboard's config-update handler without racing the HTLC
	// hot path reading it here (C-3). Callers should Load() it once per
	// logical operation for an internally consistent snapshot.
	config              *config.NodeConfigRef
	store               InterceptStore
	historyStore        history.Store
	openingService      ports.OpeningService
	feeEstimator        chain.FeeEstimator
	feeStrategy         chain.FeeStrategy
	payHashGroup        singleflight.Group
	notificationService notifications.Notifier
	nodePrivKey         interface{} // *btcec.PrivateKey for LSPS5 signing
}

func NewInterceptHandler(
	client lightning.Client,
	config *config.NodeConfigRef,
	store InterceptStore,
	historyStore history.Store,
	openingService ports.OpeningService,
	feeEstimator chain.FeeEstimator,
	feeStrategy chain.FeeStrategy,
	notificationService notifications.Notifier,
	nodePrivKey interface{},
) *Interceptor {
	nodeid, _ := hex.DecodeString(config.Load().NodePubkey)
	return &Interceptor{
		nodeid:              nodeid,
		client:              client,
		config:              config,
		store:               store,
		historyStore:        historyStore,
		openingService:      openingService,
		feeEstimator:        feeEstimator,
		feeStrategy:         feeStrategy,
		notificationService: notificationService,
		nodePrivKey:         nodePrivKey,
	}
}

func (i *Interceptor) Intercept(req core.InterceptRequest) core.InterceptResult {
	reqPaymentHashStr := hex.EncodeToString(req.PaymentHash)
	log.Debug().Msgf("Intercept: scid=%s paymentHash=%s outgoingAmount=%v outgoingExpiry=%v incomingExpiry=%v", req.Scid.ToString(), reqPaymentHashStr, req.OutgoingAmountMloki, req.OutgoingExpiry, req.IncomingExpiry)
	log.Trace().Msgf("Checking singleflight for %s", reqPaymentHashStr)
	resp, _, _ := i.payHashGroup.Do(reqPaymentHashStr, func() (interface{}, error) {
		// Snapshot once so this payment's processing sees one internally
		// consistent config even if the dashboard updates it concurrently
		// mid-flight (C-3).
		cfg := i.config.Load()
		data, err := i.resolvePayment(req)
		if err != nil {
			log.Error().Msgf("resolvePayment(%x) error: %v", req.PaymentHash, err)
			return core.InterceptResult{
				Action:      core.INTERCEPT_FAIL_HTLC_WITH_CODE,
				FailureCode: core.FAILURE_TEMPORARY_NODE_FAILURE,
			}, nil
		}

		token := data.token
		params := data.params
		paymentHash := data.paymentHash
		paymentSecret := data.paymentSecret
		destination := data.destination
		incomingAmountMloki := data.incomingAmountMloki
		outgoingAmountMloki := data.outgoingAmountMloki
		channelPoint := data.channelPoint
		tag := data.tag

		isRegistered := paymentSecret != nil
		// Sanity check. If the payment is registered, the destination is always set.
		if isRegistered && (destination == nil || len(destination) != 33) {
			log.Error().Msgf("ERROR: Payment was registered without destination. paymentHash: %s", reqPaymentHashStr)
		}

		isProbe := isRegistered && !bytes.Equal(paymentHash, req.PaymentHash)
		nextHop, err := i.client.GetPeerId(&req.Scid)
		if err != nil {
			log.Error().Msgf("GetPeerId(%s) error: %v", req.Scid.ToString(), err)
			return core.InterceptResult{
				Action: core.INTERCEPT_RESUME,
			}, nil
		}

		// If the payment was registered, but the next hop is not the destination
		// that means we are not the last hop of the payment, so we'll just forward.
		if isRegistered && nextHop != nil && !bytes.Equal(nextHop, destination) {
			log.Debug().Msgf("Intercept: paymentHash=%s routing to nextHop=%s (destination=%s)", reqPaymentHashStr, hex.EncodeToString(nextHop), hex.EncodeToString(destination))
			return core.InterceptResult{
				Action: core.INTERCEPT_RESUME,
			}, nil
		}

		// nextHop is set if the sender's scid corresponds to a known channel.
		// destination is set if the payment was registered for a channel open.
		// The 'actual' next hop will be either of those.
		if nextHop == nil {

			// If the next hop cannot be deduced from the scid, and the payment
			// is not registered, there's nothing left to be done. Just continue.
			if !isRegistered {
				log.Debug().Msgf("Intercept: paymentHash=%s skipping unregistered payment (nextHop unknown)", reqPaymentHashStr)
				return core.InterceptResult{
					Action: core.INTERCEPT_RESUME,
				}, nil
			}

			// The payment was registered, so the next hop is the registered
			// destination
			nextHop = destination
		}

		isConnected, err := i.client.IsConnected(nextHop)
		if err != nil {
			log.Error().Msgf("IsConnected(%x) error: %v", nextHop, err)
			return core.InterceptResult{
				Action:      core.INTERCEPT_FAIL_HTLC_WITH_CODE,
				FailureCode: core.FAILURE_TEMPORARY_CHANNEL_FAILURE,
			}, nil
		}

		if isProbe {
			// If this is a known probe, we'll quit early for non-connected clients.
			if !isConnected {
				log.Debug().Msgf("Intercept: paymentHash=%s probe detected but peer not connected", reqPaymentHashStr)
				return core.InterceptResult{
					Action: core.INTERCEPT_RESUME,
				}, nil
			}

			// If it's a probe, they are connected, but need a channel, we'll return
			// INCORRECT_OR_UNKNOWN_PAYMENT_DETAILS. This is an out-of-spec
			// error code, because it shouldn't be returned by an intermediate
			// node. But senders implementnig the probing-01: prefix should
			// know that the actual payment would probably succeed.
			if channelPoint == nil {
				log.Debug().Msgf("Intercept: paymentHash=%s probe detected, no channel exists", reqPaymentHashStr)
				return core.InterceptResult{
					Action:      core.INTERCEPT_FAIL_HTLC_WITH_CODE,
					FailureCode: core.FAILURE_INCORRECT_OR_UNKNOWN_PAYMENT_DETAILS,
				}, nil
			}
		}

		if isConnected {
			// If the peer is connected, notify anyway, but don't wait for
			// the result (P-4): this is a courtesy push, nothing here
			// depends on whether it actually reached the client, so
			// NotifyAsync's durable-enqueue-and-return skips the up to
			// 7s-per-webhook synchronous delivery attempt Notify does.
			// Still backgrounded via go: Enqueue is a blocking DB write,
			// and there's no reason for even that to sit inline on the
			// interception path.
			go i.notificationService.NotifyAsync(
				hex.EncodeToString(nextHop),
				reqPaymentHashStr,
				i.nodePrivKey,
			)
		} else {
			// Make sure the client is connected by potentially notifying them to come online.
			notifyResult := i.notifyAndWait(reqPaymentHashStr, nextHop, isRegistered)
			if notifyResult != nil {
				log.Debug().Msgf("Intercept: paymentHash=%s peer not notified or did not come online, continuing", reqPaymentHashStr)
				// return *notifyResult, nil
			}
		}

		// The peer is online, we can resume the htlc if it's not a channel open.
		if !isRegistered {
			return core.InterceptResult{
				Action: core.INTERCEPT_RESUME,
			}, nil
		}

		// The first htlc of a MPP will open the channel.
		if channelPoint == nil {
			// TODO: When opening_fee_params is enforced, turn this check in a temporary channel failure.
			if params == nil {
				validity := time.Hour * 24
				if cfg.JitValidUntilSeconds > 0 {
					validity = time.Duration(cfg.JitValidUntilSeconds) * time.Second
				}
				maxDelay := uint32(10000)
				if cfg.JitMaxClientToSelfDelay > 0 {
					maxDelay = cfg.JitMaxClientToSelfDelay
				}

				log.Warn().Msgf("Intercept: DEPRECATED fee mechanism used, applying defaults. paymentHash=%s", reqPaymentHashStr)
				params = &core.OpeningFeeParams{
					MinFeeMloki:          uint64(cfg.ChannelMinimumFeeMloki),
					Proportional:         uint32(cfg.ChannelFeePermyriad * 100),
					ValidUntil:           time.Now().UTC().Add(validity).Format(common.TimeFormat),
					MinLifetime:          uint32(cfg.MaxInactiveDuration / 600),
					MaxClientToSelfDelay: maxDelay,
				}
			}

			// Make sure the cltv delta is enough.
			if int64(req.IncomingExpiry)-int64(req.OutgoingExpiry) < int64(cfg.TimeLockDelta) {
				log.Debug().Msgf("Intercept: paymentHash=%s CLTV insufficient, outgoingExpiry=%v incomingExpiry=%v timeLockDelta=%v", reqPaymentHashStr, req.OutgoingExpiry, req.IncomingExpiry, cfg.TimeLockDelta)
				return core.InterceptResult{
					Action:      core.INTERCEPT_FAIL_HTLC_WITH_CODE,
					FailureCode: core.FAILURE_TEMPORARY_CHANNEL_FAILURE,
				}, nil
			}

			validUntil, err := time.Parse(common.TimeFormat, params.ValidUntil)
			if err != nil {
				log.Error().Msgf("paymentHash: %s, time.Parse(%s, %s) failed. Failing channel open: %v", reqPaymentHashStr, common.TimeFormat, params.ValidUntil, err)
				if data.orderId != "" {
					go i.notificationService.NotifyOrderStateChanged(hex.EncodeToString(nextHop), data.orderId, "FAILED", "", fmt.Sprintf("invalid valid_until format: %v", err))
				}
				return core.InterceptResult{
					Action:      core.INTERCEPT_FAIL_HTLC_WITH_CODE,
					FailureCode: core.FAILURE_TEMPORARY_CHANNEL_FAILURE,
				}, nil
			}

			// Make sure the opening_fee_params are not expired.
			// If they are expired, but the current chain fee is fine, open channel anyway.
			if time.Now().UTC().After(validUntil) {
				if !i.openingService.IsCurrentChainFeeCheaper(token, params) {
					log.Warn().Msgf("Intercept: Expired payment registration, failing. paymentHash=%s validUntil=%s", reqPaymentHashStr, params.ValidUntil)
					if data.orderId != "" {
						go i.notificationService.NotifyOrderStateChanged(hex.EncodeToString(nextHop), data.orderId, "FAILED", "", "payment registration expired")
					}
					return core.InterceptResult{
						Action:      core.INTERCEPT_FAIL_HTLC_WITH_CODE,
						FailureCode: core.FAILURE_TEMPORARY_CHANNEL_FAILURE,
					}, nil
				}

				log.Info().Msgf("Intercept: Expired registration but current fee is cheaper, proceeding. paymentHash=%s", reqPaymentHashStr)
			}

			channelPoint, err = i.openChannel(req.PaymentHash, destination, incomingAmountMloki, tag, data.isJit)
			if err != nil {
				log.Error().Msgf("paymentHash: %s, openChannel(%x, %v) err: %v", reqPaymentHashStr, destination, incomingAmountMloki, err)
				if data.orderId != "" {
					go i.notificationService.NotifyOrderStateChanged(hex.EncodeToString(nextHop), data.orderId, "FAILED", "", fmt.Sprintf("failed to open channel: %v", err))
				}
				return core.InterceptResult{
					Action:      core.INTERCEPT_FAIL_HTLC_WITH_CODE,
					FailureCode: core.FAILURE_TEMPORARY_CHANNEL_FAILURE,
				}, nil
			}
		}

		// Scale the forwarding amount proportionally for MPP parts
		// For single-part: amt = outgoingAmountMloki (since req.IncomingAmountMloki == incomingAmountMloki)
		// For MPP: each part contributes proportionally based on its incoming amount
		var bigProd, bigAmt big.Int
		amt := (bigAmt.Div(bigProd.Mul(big.NewInt(outgoingAmountMloki), big.NewInt(int64(req.IncomingAmountMloki))), big.NewInt(incomingAmountMloki))).Int64()

		deadline := time.Now().Add(60 * time.Second)

		// failChannelOpenTimeout is the shared failure path for both ways
		// this can fail to produce a usable scid: WaitChannelPointActive
		// itself timing out, or (rare) the channel becoming active with
		// no scid assigned at all.
		failChannelOpenTimeout := func(reason string) core.InterceptResult {
			log.Warn().Msgf("Intercept: Channel open timed out. paymentHash=%s (%s)", reqPaymentHashStr, reason)
			if data.orderId != "" {
				go i.notificationService.NotifyOrderStateChanged(hex.EncodeToString(nextHop), data.orderId, "FAILED", "", "channel open timed out")
			}
			return core.InterceptResult{
				Action:      core.INTERCEPT_FAIL_HTLC_WITH_CODE,
				FailureCode: core.FAILURE_TEMPORARY_CHANNEL_FAILURE,
			}
		}

		// P-1: event-driven wait (WaitChannelPointActive, backed by
		// listenChannelEvents' existing subscription) instead of polling
		// GetChannel on a fixed 1s timer.
		chanResult, err := i.client.WaitChannelPointActive(destination, *channelPoint, deadline)
		if err != nil {
			log.Debug().Msgf("Intercept: paymentHash=%s waiting for channel to open for peer %x: %v", reqPaymentHashStr, destination, err)
			return failChannelOpenTimeout(err.Error()), nil
		}

		{
			log.Info().Msgf("paymentHash: %s, channel opened successfully alias: '%v', confirmed: '%v'", reqPaymentHashStr, chanResult.AliasScid.ToString(), chanResult.ConfirmedScid.ToString())

			var scid *lightning.ShortChannelID
			if chanResult.ConfirmedScid == nil {
				if chanResult.AliasScid == nil {
					log.Error().Msgf("Error: WaitChannelPointActive: Both confirmed scid and alias scid are nil: %+v", chanResult)
					return failChannelOpenTimeout("channel became active with no scid assigned"), nil
				}
				scid = chanResult.AliasScid
			} else {
				scid = chanResult.ConfirmedScid
			}

			// Give LND a moment to propagate the new channel state to the
			// router before resuming, to avoid "no_route" errors -- even
			// with OutgoingRequestedChanId, the switch link might not be
			// fully active immediately.
			//
			// Poll IsChannelRoutable (GetChanInfo), not QueryRoutes.
			// Root-caused live (2026-07-16): for a just-opened zero-conf
			// channel, QueryRoutes' pathfinding reads from flnd's in-memory
			// graph cache, which can lag 60-90+s behind the persisted graph
			// DB -- confirmed directly via GetChanInfo showing both edge
			// policies present within the same second the channel
			// activates, on every connection/flnd-branch/devnet-state
			// combination tested (ruling out flspd's own connection
			// management as the cause). Upstream LND's own zero-conf
			// channel itest (testZeroConfChannelOpen) never calls
			// QueryRoutes for this readiness check either -- it polls
			// DescribeGraph for both directional policies, which (like
			// GetChanInfo) reads straight from the graph DB rather than the
			// pathfinding cache. See BACKLOG.md's LSPS2 zero-conf
			// balance-sync race writeup for the full investigation.
			routeTimeout := time.After(60 * time.Second)
			routeTicker := time.NewTicker(500 * time.Millisecond)
			defer routeTicker.Stop()

			var targetScid uint64
			if scid != nil {
				targetScid = uint64(*scid)
			}

		RouteLoop:
			for {
				routable, err := i.client.IsChannelRoutable(targetScid)
				switch {
				case err != nil:
					log.Trace().Err(err).Msgf("Intercept: IsChannelRoutable failed (waiting for sync), paymentHash=%s", reqPaymentHashStr)
				case routable:
					log.Debug().Msgf("Intercept: paymentHash=%s channel %d routable, resuming", reqPaymentHashStr, targetScid)
					break RouteLoop
				}

				select {
				case <-routeTimeout:
					log.Warn().Msgf("Intercept: IsChannelRoutable timed out using Best Effort for paymentHash=%s, resuming anyway", reqPaymentHashStr)
					break RouteLoop
				case <-routeTicker.C:
					continue
				}
			}

			err = i.historyStore.UpdateChannels(context.TODO(), []*history.ChannelUpdate{{
				NodeID:        i.nodeid,
				PeerId:        destination,
				AliasScid:     chanResult.AliasScid,
				ConfirmedScid: chanResult.ConfirmedScid,
				ChannelPoint:  channelPoint,
				LastUpdate:    time.Now(),
			}})
			if err != nil {
				// Don't break here, this is not critical.
				log.Error().Msgf("paymentHash: %s, failed to insert newly opened channel in history store. %v: %v", reqPaymentHashStr, channelPoint.String(), err)
			}

			err = i.historyStore.AddOpenChannelHtlc(context.TODO(), &history.OpenChannelHtlc{
				NodeId:              i.nodeid,
				PeerId:              destination,
				ChannelPoint:        channelPoint,
				OriginalAmountMloki: req.OutgoingAmountMloki,
				ForwardAmountMloki:  uint64(amt),
				IncomingAmountMloki: req.IncomingAmountMloki,
				ForwardTime:         time.Now(),
			})
			if err != nil {
				// Don't break here, this is not critical.
				log.Error().Msgf(
					"paymentHash: %s, failed to insert htlc used for channel open in history store: channel: %v,"+
						" original amount: %v, forward amount: %v, error: %v",
					reqPaymentHashStr,
					channelPoint.String(),
					req.OutgoingAmountMloki,
					amt,
					err,
				)
			}

			if data.orderId != "" {
				go i.notificationService.NotifyOrderStateChanged(
					hex.EncodeToString(destination),
					data.orderId,
					"COMPLETED",
					channelPoint.String(),
					"",
				)
			}

			return core.InterceptResult{
				Action:           core.INTERCEPT_RESUME_WITH_ONION,
				Destination:      destination,
				ChannelPoint:     channelPoint,
				Scid:             *scid,
				PaymentSecret:    paymentSecret,
				AmountMloki:      uint64(amt),
				TotalAmountMloki: uint64(outgoingAmountMloki),
			}, nil
		}
	})

	return resp.(core.InterceptResult)
}

func (i *Interceptor) notifyAndWait(reqPaymentHashStr string, nextHop []byte, isRegistered bool) *core.InterceptResult {
	// If not connected, send a notification to the registered
	// notification service for this client if available.
	notified, err := i.notificationService.Notify(
		hex.EncodeToString(nextHop),
		reqPaymentHashStr,
		i.nodePrivKey,
	)

	// If this errors or the client is not notified, the client
	// is offline or unknown. We'll resume the HTLC (which will
	// result in UNKOWN_NEXT_PEER)
	if err != nil || !notified {
		return &core.InterceptResult{
			Action: core.INTERCEPT_RESUME,
		}
	}

	log.Debug().Msgf("Intercept: paymentHash=%s notified peer %x of pending HTLC", reqPaymentHashStr, nextHop)
	d, err := time.ParseDuration(i.config.Load().NotificationTimeout)
	if err != nil {
		log.Warn().Msg("Intercept: NotificationTimeout not set, using default 1m")
		d = time.Minute
	}
	timeout := time.Now().Add(d)

	// Wait for a while to allow the client to come online.
	err = i.client.WaitOnline(nextHop, timeout)

	// If there's an error waiting, resume the htlc. It will
	// probably fail with UNKNOWN_NEXT_PEER.
	if err != nil {
		log.Debug().Msgf(
			"Intercept: paymentHash=%s waiting for peer %x to come online failed: %v",
			reqPaymentHashStr,
			nextHop,
			err,
		)
		return &core.InterceptResult{
			Action: core.INTERCEPT_RESUME,
		}
	}

	log.Info().Msgf("paymentHash %s, Peer %x is back online. Continue htlc.", reqPaymentHashStr, nextHop)
	// At this point we know a few things.
	// - This is either a channel partner or a registered payment
	// - they were offline
	// - They got notified about the htlc
	// - They came back online
	// So if this payment was not registered, this is a channel
	// partner and we have to wait for the channel to become active
	// before we can forward.
	if !isRegistered {
		err = i.client.WaitChannelActive(nextHop, timeout)
		if err != nil {
			log.Debug().Msgf(
				"Intercept: paymentHash=%s waiting for channel with %x to become active failed: %v",
				reqPaymentHashStr,
				nextHop,
				err,
			)
			return &core.InterceptResult{
				Action: core.INTERCEPT_RESUME,
			}
		}
	}

	return nil
}

func (i *Interceptor) openChannel(paymentHash, destination []byte, incomingAmountMloki int64, tag *string, isJit bool) (*wire.OutPoint, error) {
	// Snapshot once so this channel-open decision is made against one
	// internally consistent config even if the dashboard updates it
	// concurrently mid-flight (C-3).
	cfg := i.config.Load()

	capacity := incomingAmountMloki/1000 + cfg.AdditionalChannelCapacity
	if capacity == cfg.PublicChannelAmount {
		capacity++
	}

	fee, err := i.feeEstimator.EstimateFeeRate(
		context.Background(),
		i.feeStrategy,
	)
	if err != nil || fee == nil {
		log.Error().Msgf("Error estimating chain fee, fallback to target conf: %v", err)
		fee = &chain.FeeEstimation{
			TargetConf: &cfg.TargetConf,
		}
	}

	// Enforce Minimum Channel Capacity (Physical Floor)
	if uint64(capacity) < cfg.MinChannelCapacityLoki {
		return nil, fmt.Errorf("channel capacity %d is smaller than minimum %d (network floor)", capacity, cfg.MinChannelCapacityLoki)
	}

	// Enforce Business Limits for JIT
	if cfg.JitEnabled && isJit {
		// 1. Check Business Floor (Minimum Channel Size)
		if cfg.JitMinChannelBalanceLoki != nil && uint64(capacity) < *cfg.JitMinChannelBalanceLoki {
			return nil, fmt.Errorf("channel capacity %d is smaller than business minimum %d", capacity, *cfg.JitMinChannelBalanceLoki)
		}

		// 2. Check Maximum Payment Size
		if cfg.JitMaxPaymentSizeMloki > 0 && uint64(incomingAmountMloki) > cfg.JitMaxPaymentSizeMloki {
			return nil, fmt.Errorf("payment size %d exceeds maximum allowed %d", incomingAmountMloki, cfg.JitMaxPaymentSizeMloki)
		}
	}

	minConfs := cfg.MinConfs
	if cfg.JitEnabled && isJit {
		log.Debug().Msgf("Intercept: JIT enabled, overriding MinConfs to 0 for paymentHash=%x", paymentHash)
		zero := uint32(0)
		minConfs = &zero
	}

	log.Info().Msgf(
		"Opening zero conf channel. Paymenthash: %x, Destination: %x, capacity: %v, fee: %s, targetConf %v",
		paymentHash,
		destination,
		capacity,
		fee.String(),
		fee.TargetConf,
	)

	// For JIT channels, use zero routing fees to avoid circular dependency:
	// Client pays to their own invoice, so charging routing fees would require
	// them to pay more than the invoice amount, causing FeeInsufficient errors.
	var routingPolicy *lightning.RoutingPolicy
	if isJit {
		routingPolicy = &lightning.RoutingPolicy{
			Ppm:           0, // Zero fee rate for JIT
			BaseMloki:     0, // Zero base fee for JIT
			TimeLockDelta: cfg.TimeLockDelta,
		}
	} else {
		routingPolicy = &lightning.RoutingPolicy{
			Ppm:           uint32(cfg.FeeRate * 1_000_000),
			BaseMloki:     cfg.BaseFeeMloki,
			TimeLockDelta: cfg.TimeLockDelta,
		}
	}

	channelPoint, err := i.client.OpenChannel(&lightning.OpenChannelRequest{
		Destination:     destination,
		CapacityLoki:    uint64(capacity),
		MinConfs:        minConfs,
		FeeLokiPerVByte: fee.LokiPerVByte,
		TargetConf:      fee.TargetConf,
		RoutingPolicy:   routingPolicy,
		ZeroConf:        cfg.JitEnabled && isJit,
	})
	if err != nil {
		log.Error().Msgf("paymenthash %x, client.OpenChannelSync(%x, %v) error: %v", paymentHash, destination, capacity, err)
		return nil, err
	}
	sendOpenChannelDiscordNotification(
		paymentHash,
		incomingAmountMloki,
		destination,
		capacity,
		channelPoint.String(),
		tag,
	)
	err = i.store.SetFundingTx(paymentHash, channelPoint)
	return channelPoint, err
}

// computeJitOutgoingAmount derives the amount to forward for an LSPS2 JIT
// payment, given the gross amount the sender paid (paymentSizeMloki) and
// the opening-fee params. Lightning fees are charged on the forwarded
// (outgoing) amount, so given input = outgoing + outgoing*rate, solving
// for outgoing yields outgoing = input / (1 + rate). minFeeMloki acts as
// a floor: if the proportional fee implied by that formula falls below
// it, the floor is charged instead and outgoing is reduced accordingly.
func computeJitOutgoingAmount(paymentSizeMloki, minFeeMloki int64, proportional uint32) (outgoing int64, fee int64) {
	rate := float64(proportional) / 1_000_000.0

	outgoing = int64(float64(paymentSizeMloki) / (1.0 + rate))
	fee = paymentSizeMloki - outgoing

	if fee < minFeeMloki {
		fee = minFeeMloki
		outgoing = paymentSizeMloki - fee
	}

	if outgoing < 0 {
		outgoing = 0
	}

	return outgoing, fee
}

type interceptData struct {
	token               string
	orderId             string
	params              *core.OpeningFeeParams
	paymentHash         []byte
	paymentSecret       []byte
	destination         []byte
	incomingAmountMloki int64
	outgoingAmountMloki int64
	channelPoint        *wire.OutPoint
	tag                 *string
	isJit               bool
}

func (i *Interceptor) resolvePayment(req core.InterceptRequest) (*interceptData, error) {
	token, orderId, params, paymentHash, paymentSecret, destination, incomingAmountMloki, outgoingAmountMloki, channelPoint, tag, err := i.store.PaymentInfo(req.PaymentHash)
	isJit := false

	// If err is nil but paymentHash/token is empty, it means not found in payments table.
	// We should treat this as "need fallback" for LSPS2.
	if err != nil || (len(paymentHash) == 0 && token == "") {
		// Try LSPS2 lookup using SCID
		lsps2Order, err2 := i.store.GetLsps2Order(uint64(req.Scid))
		if err2 == nil && lsps2Order != nil {
			token = lsps2Order.Token
			destination, _ = hex.DecodeString(lsps2Order.PeerId)
			params = &core.OpeningFeeParams{
				MinFeeMloki:          lsps2Order.ParamsMinFeeMloki,
				Proportional:         lsps2Order.ParamsProportional,
				ValidUntil:           lsps2Order.ParamsValidUntil,
				MinLifetime:          lsps2Order.ParamsMinLifetime,
				MaxClientToSelfDelay: lsps2Order.ParamsMaxClientToSelfDelay,
				Promise:              lsps2Order.ParamsPromise,
			}
			paymentHash = req.PaymentHash
			paymentSecret = []byte("lsps2-jit") // Mark as registered
			if lsps2Order.PaymentSizeMloki != nil {
				paymentSize := int64(*lsps2Order.PaymentSizeMloki)
				minFee := int64(lsps2Order.ParamsMinFeeMloki)

				var fee int64
				outgoingAmountMloki, fee = computeJitOutgoingAmount(paymentSize, minFee, lsps2Order.ParamsProportional)

				log.Info().Msgf("LSPS2 JIT: PaymentSize=%d, Fee=%d, Outgoing=%d", paymentSize, fee, outgoingAmountMloki)

				// Validation: Ensure the sender paid enough to cover the PaymentSize (which fee is calculated from)
				if int64(req.IncomingAmountMloki) < paymentSize {
					log.Warn().Msgf("LSPS2 JIT: Underpayment detected. PaymentSize=%d > Incoming=%d", paymentSize, req.IncomingAmountMloki)
					return nil, fmt.Errorf("underpayment: incoming amount %d is less than required payment size %d", req.IncomingAmountMloki, paymentSize)
				}
			}
			incomingAmountMloki = int64(req.IncomingAmountMloki)
			isJit = true

			orderId = lsps2Order.Id

			// Register the payment
			err = i.store.RegisterPayment(token, orderId, params, destination, paymentHash, paymentSecret, incomingAmountMloki, outgoingAmountMloki, "")
			if err != nil {
				log.Error().Msgf("Failed to register LSPS2 payment: %v", err)
				return nil, err
			}
		} else {
			// Check if we can identify the destination from the SCID to look up LSPS1 orders
			nextHop, errHop := i.client.GetPeerId(&req.Scid)
			if errHop == nil && nextHop != nil {
				clientNodeId := hex.EncodeToString(nextHop)

				// Check for PAID LSPS1 orders for this peer
				lsps1Order, errOrder := i.store.GetPaidLsps1Order(clientNodeId)
				if errOrder == nil && lsps1Order != nil {
					log.Info().Msgf("Found PAID LSPS1 order %s for peer %s, activating JIT", lsps1Order.OrderId, clientNodeId)

					// Found a matching order! Activate JIT channel opening.
					token = lsps1Order.Token
					destination = nextHop

					// Use default fee params for now as LSPS1 params aren't stored in DB in same format
					// TODO: Store actual params from order creation
					cfg := i.config.Load()
					validity := time.Hour * 24
					if cfg.JitValidUntilSeconds > 0 {
						validity = time.Duration(cfg.JitValidUntilSeconds) * time.Second
					}
					maxDelay := uint32(10000)
					if cfg.JitMaxClientToSelfDelay > 0 {
						maxDelay = cfg.JitMaxClientToSelfDelay
					}

					params = &core.OpeningFeeParams{
						MinFeeMloki:          uint64(cfg.ChannelMinimumFeeMloki),
						Proportional:         uint32(cfg.ChannelFeePermyriad * 100),
						ValidUntil:           time.Now().Add(validity).Format(common.TimeFormat),
						MinLifetime:          uint32(cfg.MaxInactiveDuration / 600),
						MaxClientToSelfDelay: maxDelay,
					}

					paymentHash = req.PaymentHash
					paymentSecret = []byte("lsps1-jit")

					// Set amounts
					if lsps1Order.ClientBalanceLoki > 0 {
						outgoingAmountMloki = int64(lsps1Order.ClientBalanceLoki * 1000)
					}
					incomingAmountMloki = int64(req.IncomingAmountMloki)
					isJit = true
					lsps1Tag := "lsps1.v1"
					tag = &lsps1Tag

					orderId = lsps1Order.OrderId

					// Register this specific payment hash so we handle retries correctly
					err = i.store.RegisterPayment(token, orderId, params, destination, paymentHash, paymentSecret, incomingAmountMloki, outgoingAmountMloki, lsps1Tag)
					if err != nil {
						log.Error().Msgf("Failed to register LSPS1 payment: %v", err)
						return nil, err
					}
				}
			}
		}
	}

	if err != nil {
		return nil, err
	}

	return &interceptData{
		token:               token,
		orderId:             orderId,
		params:              params,
		paymentHash:         paymentHash,
		paymentSecret:       paymentSecret,
		destination:         destination,
		incomingAmountMloki: incomingAmountMloki,
		outgoingAmountMloki: outgoingAmountMloki,
		channelPoint:        channelPoint,
		tag:                 tag,
		isJit:               isJit,
	}, nil
}
