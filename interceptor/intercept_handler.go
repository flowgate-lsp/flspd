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
	"github.com/flowgate-lsp/flspd/config"
	"github.com/flowgate-lsp/flspd/format"
	"github.com/flowgate-lsp/flspd/history"
	"github.com/flowgate-lsp/flspd/lightning"
	"github.com/flowgate-lsp/flspd/notifications"
	"github.com/rs/zerolog/log"
	"golang.org/x/sync/singleflight"
)

type Interceptor struct {
	nodeid              []byte
	client              lightning.Client
	config              *config.NodeConfig
	store               InterceptStore
	historyStore        history.Store
	openingService      common.OpeningService
	feeEstimator        chain.FeeEstimator
	feeStrategy         chain.FeeStrategy
	payHashGroup        singleflight.Group
	notificationService notifications.Notifier
	nodePrivKey         interface{} // *btcec.PrivateKey for LSPS5 signing
}

func NewInterceptHandler(
	client lightning.Client,
	config *config.NodeConfig,
	store InterceptStore,
	historyStore history.Store,
	openingService common.OpeningService,
	feeEstimator chain.FeeEstimator,
	feeStrategy chain.FeeStrategy,
	notificationService notifications.Notifier,
	nodePrivKey interface{},
) *Interceptor {
	nodeid, _ := hex.DecodeString(config.NodePubkey)
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

func (i *Interceptor) Intercept(req common.InterceptRequest) common.InterceptResult {
	reqPaymentHashStr := hex.EncodeToString(req.PaymentHash)
	log.Debug().Msgf("Intercept: scid=%s paymentHash=%s outgoingAmount=%v outgoingExpiry=%v incomingExpiry=%v", req.Scid.ToString(), reqPaymentHashStr, req.OutgoingAmountMloki, req.OutgoingExpiry, req.IncomingExpiry)
	log.Trace().Msgf("Checking singleflight for %s", reqPaymentHashStr)
	resp, _, _ := i.payHashGroup.Do(reqPaymentHashStr, func() (interface{}, error) {
		data, err := i.resolvePayment(req)
		if err != nil {
			log.Error().Msgf("resolvePayment(%x) error: %v", req.PaymentHash, err)
			return common.InterceptResult{
				Action:      common.INTERCEPT_FAIL_HTLC_WITH_CODE,
				FailureCode: common.FAILURE_TEMPORARY_NODE_FAILURE,
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
			return common.InterceptResult{
				Action: common.INTERCEPT_RESUME,
			}, nil
		}

		// If the payment was registered, but the next hop is not the destination
		// that means we are not the last hop of the payment, so we'll just forward.
		if isRegistered && nextHop != nil && !bytes.Equal(nextHop, destination) {
			log.Debug().Msgf("Intercept: paymentHash=%s routing to nextHop=%s (destination=%s)", reqPaymentHashStr, hex.EncodeToString(nextHop), hex.EncodeToString(destination))
			return common.InterceptResult{
				Action: common.INTERCEPT_RESUME,
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
				return common.InterceptResult{
					Action: common.INTERCEPT_RESUME,
				}, nil
			}

			// The payment was registered, so the next hop is the registered
			// destination
			nextHop = destination
		}

		isConnected, err := i.client.IsConnected(nextHop)
		if err != nil {
			log.Error().Msgf("IsConnected(%x) error: %v", nextHop, err)
			return common.InterceptResult{
				Action:      common.INTERCEPT_FAIL_HTLC_WITH_CODE,
				FailureCode: common.FAILURE_TEMPORARY_CHANNEL_FAILURE,
			}, nil
		}

		if isProbe {
			// If this is a known probe, we'll quit early for non-connected clients.
			if !isConnected {
				log.Debug().Msgf("Intercept: paymentHash=%s probe detected but peer not connected", reqPaymentHashStr)
				return common.InterceptResult{
					Action: common.INTERCEPT_RESUME,
				}, nil
			}

			// If it's a probe, they are connected, but need a channel, we'll return
			// INCORRECT_OR_UNKNOWN_PAYMENT_DETAILS. This is an out-of-spec
			// error code, because it shouldn't be returned by an intermediate
			// node. But senders implementnig the probing-01: prefix should
			// know that the actual payment would probably succeed.
			if channelPoint == nil {
				log.Debug().Msgf("Intercept: paymentHash=%s probe detected, no channel exists", reqPaymentHashStr)
				return common.InterceptResult{
					Action:      common.INTERCEPT_FAIL_HTLC_WITH_CODE,
					FailureCode: common.FAILURE_INCORRECT_OR_UNKNOWN_PAYMENT_DETAILS,
				}, nil
			}
		}

		if isConnected {
			// If the peer is connected, notify anyway, but don't wait for the result.
			go i.notificationService.Notify(
				hex.EncodeToString(nextHop),
				reqPaymentHashStr,
				i.nodePrivKey,
			)
		} else {
			// Make sure the client is connected by potentially notifying them to come online.
			notifyResult := i.notifyAndWait(reqPaymentHashStr, nextHop, isRegistered)
			if notifyResult != nil {
				log.Debug().Msgf("Intercept: paymentHash=%s peer offline, notification sent", reqPaymentHashStr)
				// return *notifyResult, nil
			}
		}

		// The peer is online, we can resume the htlc if it's not a channel open.
		if !isRegistered {
			return common.InterceptResult{
				Action: common.INTERCEPT_RESUME,
			}, nil
		}

		// The first htlc of a MPP will open the channel.
		if channelPoint == nil {
			// TODO: When opening_fee_params is enforced, turn this check in a temporary channel failure.
			if params == nil {
				validity := time.Hour * 24
				if i.config.JitValidUntilSeconds > 0 {
					validity = time.Duration(i.config.JitValidUntilSeconds) * time.Second
				}
				maxDelay := uint32(10000)
				if i.config.JitMaxClientToSelfDelay > 0 {
					maxDelay = i.config.JitMaxClientToSelfDelay
				}

				log.Warn().Msgf("Intercept: DEPRECATED fee mechanism used, applying defaults. paymentHash=%s", reqPaymentHashStr)
				params = &common.OpeningFeeParams{
					MinFeeMloki:          uint64(i.config.ChannelMinimumFeeMloki),
					Proportional:         uint32(i.config.ChannelFeePermyriad * 100),
					ValidUntil:           time.Now().UTC().Add(validity).Format(format.TIME_FORMAT),
					MinLifetime:          uint32(i.config.MaxInactiveDuration / 600),
					MaxClientToSelfDelay: maxDelay,
				}
			}

			// Make sure the cltv delta is enough.
			if int64(req.IncomingExpiry)-int64(req.OutgoingExpiry) < int64(i.config.TimeLockDelta) {
				log.Debug().Msgf("Intercept: paymentHash=%s CLTV insufficient, outgoingExpiry=%v incomingExpiry=%v timeLockDelta=%v", reqPaymentHashStr, req.OutgoingExpiry, req.IncomingExpiry, i.config.TimeLockDelta)
				return common.InterceptResult{
					Action:      common.INTERCEPT_FAIL_HTLC_WITH_CODE,
					FailureCode: common.FAILURE_TEMPORARY_CHANNEL_FAILURE,
				}, nil
			}

			validUntil, err := time.Parse(format.TIME_FORMAT, params.ValidUntil)
			if err != nil {
				log.Error().Msgf("paymentHash: %s, time.Parse(%s, %s) failed. Failing channel open: %v", reqPaymentHashStr, format.TIME_FORMAT, params.ValidUntil, err)
				if data.orderId != "" {
					go i.notificationService.NotifyOrderStateChanged(hex.EncodeToString(nextHop), data.orderId, "FAILED", "", fmt.Sprintf("invalid valid_until format: %v", err))
				}
				return common.InterceptResult{
					Action:      common.INTERCEPT_FAIL_HTLC_WITH_CODE,
					FailureCode: common.FAILURE_TEMPORARY_CHANNEL_FAILURE,
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
					return common.InterceptResult{
						Action:      common.INTERCEPT_FAIL_HTLC_WITH_CODE,
						FailureCode: common.FAILURE_TEMPORARY_CHANNEL_FAILURE,
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
				return common.InterceptResult{
					Action:      common.INTERCEPT_FAIL_HTLC_WITH_CODE,
					FailureCode: common.FAILURE_TEMPORARY_CHANNEL_FAILURE,
				}, nil
			}
		}

		// Scale the forwarding amount proportionally for MPP parts
		// For single-part: amt = outgoingAmountMloki (since req.IncomingAmountMloki == incomingAmountMloki)
		// For MPP: each part contributes proportionally based on its incoming amount
		var bigProd, bigAmt big.Int
		amt := (bigAmt.Div(bigProd.Mul(big.NewInt(outgoingAmountMloki), big.NewInt(int64(req.IncomingAmountMloki))), big.NewInt(incomingAmountMloki))).Int64()

		deadline := time.Now().Add(60 * time.Second)

		for {
			chanResult, _ := i.client.GetChannel(destination, *channelPoint)
			if chanResult != nil {
				log.Info().Msgf("paymentHash: %s, channel opened successfully alias: '%v', confirmed: '%v'", reqPaymentHashStr, chanResult.AliasScid.ToString(), chanResult.ConfirmedScid.ToString())

				var scid *lightning.ShortChannelID
				if chanResult.ConfirmedScid == nil {
					if chanResult.AliasScid == nil {
						log.Error().Msgf("Error: GetChannel: Both confirmed scid and alias scid are nil: %+v", chanResult)
						<-time.After(1 * time.Second)
						continue
					}
					scid = chanResult.AliasScid
				} else {
					scid = chanResult.ConfirmedScid
				}

				// Give LND a moment to propagate the new channel state to the router
				// to avoid "no_route" errors. Even with OutgoingRequestedChanId,
				// the switch link might not be fully active immediately.
				// Verify that the route is available in the graph
				// This avoids "no_route" errors if the switch link is not fully active yet.
				// Use a minimal amount for verification to check strict connectivity/existence.
				// We avoid checking the full amount here because the Router's view of likely-zero-conf
				// channels might show 0 balance initially, causing "insufficient local balance" errors
				// even if the Switch shows full bandwidth. We trust the Switch to handle the full amount check on resume.
				routeParamsAmount := uint64(incomingAmountMloki)

				// Use a generous timeout (60s) to wait for the Router to sync the ZC balance.
				// "Insufficient local balance" means we must wait longer, not give up or bypass.
				routeTimeout := time.After(60 * time.Second)
				routeTicker := time.NewTicker(500 * time.Millisecond) // Poll faster
				defer routeTicker.Stop()

				var targetScid uint64
				if scid != nil {
					targetScid = uint64(*scid)
				}

			RouteLoop:
				for {
					routes, err := i.client.QueryRoutes(destination, routeParamsAmount, targetScid)
					if err == nil {
						for _, r := range routes {
							if len(r.Hops) > 0 {
								// Strict check: Ensure the route uses the newly opened channel
								if r.Hops[0].ChanId == targetScid {
									log.Debug().Msgf("Intercept: paymentHash=%s route found via SCID %d, resuming", reqPaymentHashStr, targetScid)
									break RouteLoop
								}
								// If we found a route but via different SCID, log it
								log.Debug().Msgf("Intercept: found route via SCID %d, waiting for target %d", r.Hops[0].ChanId, targetScid)
							}
						}
					} else {
						// Log error but continue polling. "insufficient local balance" is a common temporary state.
						log.Trace().Err(err).Msgf("Intercept: QueryRoutes failed (waiting for sync), paymentHash=%s", reqPaymentHashStr)
					}

					select {
					case <-routeTimeout:
						log.Warn().Msgf("Intercept: QueryRoutes timed out using Best Effort for paymentHash=%s, resuming anyway", reqPaymentHashStr)
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

				return common.InterceptResult{
					Action:           common.INTERCEPT_RESUME_WITH_ONION,
					Destination:      destination,
					ChannelPoint:     channelPoint,
					Scid:             *scid,
					PaymentSecret:    paymentSecret,
					AmountMloki:      uint64(amt),
					TotalAmountMloki: uint64(outgoingAmountMloki),
				}, nil
			}

			log.Debug().Msgf("Intercept: paymentHash=%s waiting for channel to open for peer %x", reqPaymentHashStr, destination)
			if time.Now().After(deadline) {
				log.Debug().Msgf("Intercept: paymentHash=%s getChannel timeout, stopping retries", reqPaymentHashStr)
				break
			}
			<-time.After(1 * time.Second)
		}

		log.Warn().Msgf("Intercept: Channel open timed out. paymentHash=%s", reqPaymentHashStr)
		if data.orderId != "" {
			go i.notificationService.NotifyOrderStateChanged(hex.EncodeToString(nextHop), data.orderId, "FAILED", "", "channel open timed out")
		}
		return common.InterceptResult{
			Action:      common.INTERCEPT_FAIL_HTLC_WITH_CODE,
			FailureCode: common.FAILURE_TEMPORARY_CHANNEL_FAILURE,
		}, nil
	})

	return resp.(common.InterceptResult)
}

func (i *Interceptor) notifyAndWait(reqPaymentHashStr string, nextHop []byte, isRegistered bool) *common.InterceptResult {
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
		return &common.InterceptResult{
			Action: common.INTERCEPT_RESUME,
		}
	}

	log.Debug().Msgf("Intercept: paymentHash=%s notified peer %x of pending HTLC", reqPaymentHashStr, nextHop)
	d, err := time.ParseDuration(i.config.NotificationTimeout)
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
		return &common.InterceptResult{
			Action: common.INTERCEPT_RESUME,
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
			return &common.InterceptResult{
				Action: common.INTERCEPT_RESUME,
			}
		}
	}

	return nil
}

func (i *Interceptor) openChannel(paymentHash, destination []byte, incomingAmountMloki int64, tag *string, isJit bool) (*wire.OutPoint, error) {
	capacity := incomingAmountMloki/1000 + i.config.AdditionalChannelCapacity
	if capacity == i.config.PublicChannelAmount {
		capacity++
	}

	fee, err := i.feeEstimator.EstimateFeeRate(
		context.Background(),
		i.feeStrategy,
	)
	if err != nil || fee == nil {
		log.Error().Msgf("Error estimating chain fee, fallback to target conf: %v", err)
		fee = &chain.FeeEstimation{
			TargetConf: &i.config.TargetConf,
		}
	}

	// Enforce Minimum Channel Capacity (Physical Floor)
	if uint64(capacity) < i.config.MinChannelCapacityLoki {
		return nil, fmt.Errorf("channel capacity %d is smaller than minimum %d (network floor)", capacity, i.config.MinChannelCapacityLoki)
	}

	// Enforce Business Limits for JIT
	if i.config.JitEnabled && isJit {
		// 1. Check Business Floor (Minimum Channel Size)
		if i.config.JitMinChannelBalanceLoki != nil && uint64(capacity) < *i.config.JitMinChannelBalanceLoki {
			return nil, fmt.Errorf("channel capacity %d is smaller than business minimum %d", capacity, *i.config.JitMinChannelBalanceLoki)
		}

		// 2. Check Maximum Payment Size
		if i.config.JitMaxPaymentSizeMloki > 0 && uint64(incomingAmountMloki) > i.config.JitMaxPaymentSizeMloki {
			return nil, fmt.Errorf("payment size %d exceeds maximum allowed %d", incomingAmountMloki, i.config.JitMaxPaymentSizeMloki)
		}
	}

	minConfs := i.config.MinConfs
	if i.config.JitEnabled && isJit {
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
			TimeLockDelta: i.config.TimeLockDelta,
		}
	} else {
		routingPolicy = &lightning.RoutingPolicy{
			Ppm:           uint32(i.config.FeeRate * 1_000_000),
			BaseMloki:     i.config.BaseFeeMloki,
			TimeLockDelta: i.config.TimeLockDelta,
		}
	}

	channelPoint, err := i.client.OpenChannel(&lightning.OpenChannelRequest{
		Destination:     destination,
		CapacityLoki:    uint64(capacity),
		MinConfs:        minConfs,
		FeeLokiPerVByte: fee.LokiPerVByte,
		TargetConf:      fee.TargetConf,
		RoutingPolicy:   routingPolicy,
		ZeroConf:        i.config.JitEnabled && isJit,
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

type interceptData struct {
	token               string
	orderId             string
	params              *common.OpeningFeeParams
	paymentHash         []byte
	paymentSecret       []byte
	destination         []byte
	incomingAmountMloki int64
	outgoingAmountMloki int64
	channelPoint        *wire.OutPoint
	tag                 *string
	isJit               bool
}

func (i *Interceptor) resolvePayment(req common.InterceptRequest) (*interceptData, error) {
	token, orderId, params, paymentHash, paymentSecret, destination, incomingAmountMloki, outgoingAmountMloki, channelPoint, tag, err := i.store.PaymentInfo(req.PaymentHash)
	isJit := false

	// If err is nil but paymentHash/token is empty, it means not found in payments table.
	// We should treat this as "need fallback" for LSPS2.
	if err != nil || (len(paymentHash) == 0 && token == "") {
		// Try LSPS2 lookup using SCID
		lsps2Order, err2 := i.store.GetLsps2Order(uint64(req.Scid))
		if err2 == nil && lsps2Order != nil {
			err = nil
			token = lsps2Order.Token
			destination, _ = hex.DecodeString(lsps2Order.PeerId)
			params = &common.OpeningFeeParams{
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

				// Calculate Fee matching computeOpeningFee in lsps/server.go
				minFee := int64(lsps2Order.ParamsMinFeeMloki)

				// Fee Logic:
				// Standard Lightning fees are charged on the FORWARDED (outgoing) amount.
				// Fee = Outgoing * Rate + Base (we treat MinFee as a floor here, effectively base=0 usually).
				// We know Input (PaymentSize). We need to find Outgoing such that:
				// Outgoing + Fee(Outgoing) <= Input
				// Outgoing + Outgoing * Rate = Input
				// Outgoing * (1 + Rate) = Input
				// Outgoing = Input / (1 + Rate)

				rate := float64(lsps2Order.ParamsProportional) / 1_000_000.0

				// Calculate Outgoing based on proportional fee
				// outgoing = input / (1 + rate)
				outgoingAmountMloki = int64(float64(paymentSize) / (1.0 + rate))

				// Calculate the implied fee
				fee := paymentSize - outgoingAmountMloki

				// Enforce MinFee floor
				if fee < minFee {
					fee = minFee
					outgoingAmountMloki = paymentSize - fee
				}

				if outgoingAmountMloki < 0 {
					outgoingAmountMloki = 0
				}

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
					err = nil
					token = lsps1Order.Token
					destination = nextHop

					// Use default fee params for now as LSPS1 params aren't stored in DB in same format
					// TODO: Store actual params from order creation
					validity := time.Hour * 24
					if i.config.JitValidUntilSeconds > 0 {
						validity = time.Duration(i.config.JitValidUntilSeconds) * time.Second
					}
					maxDelay := uint32(10000)
					if i.config.JitMaxClientToSelfDelay > 0 {
						maxDelay = i.config.JitMaxClientToSelfDelay
					}

					params = &common.OpeningFeeParams{
						MinFeeMloki:          uint64(i.config.ChannelMinimumFeeMloki),
						Proportional:         uint32(i.config.ChannelFeePermyriad * 100),
						ValidUntil:           time.Now().Add(validity).Format(format.TIME_FORMAT),
						MinLifetime:          uint32(i.config.MaxInactiveDuration / 600),
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
