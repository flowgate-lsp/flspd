package lnd

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/flokiorg/flnd/lnrpc"
	"github.com/flokiorg/flnd/lnrpc/chainrpc"
	"github.com/flokiorg/flnd/lnrpc/routerrpc"
	"github.com/flokiorg/go-flokicoin/chaincfg/chainhash"
	"github.com/flokiorg/go-flokicoin/wire"
	"github.com/flowgate-lsp/flspd/config"
	"github.com/flowgate-lsp/flspd/lightning"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/status"
)

type LndClient struct {
	client              lnrpc.LightningClient
	routerClient        routerrpc.RouterClient
	chainNotifierClient chainrpc.ChainNotifierClient
	conn                *grpc.ClientConn
	listenerCtx         context.Context
	listenerCancel      context.CancelFunc
	peersubs            map[string]map[uint64]chan struct{}
	chansubs            map[string]map[uint64]chan struct{}
	submtx              sync.RWMutex
	index               uint64
}

func NewLndClient(conf *config.LndConfig) (*LndClient, error) {
	_, err := hex.DecodeString(conf.Macaroon)
	if err != nil {
		return nil, fmt.Errorf("failed to decode macaroon: %w", err)
	}

	// Creds file to connect to LND gRPC
	// User requested specific format: Hex-encoded PEM string.
	// We hex decode the string, then parse it as a PEM certificate.
	certBytes, err := hex.DecodeString(conf.Cert)
	if err != nil {
		return nil, fmt.Errorf("failed to decode certificate (expecting hex-encoded PEM): %w", err)
	}

	cp := x509.NewCertPool()
	if !cp.AppendCertsFromPEM(certBytes) {
		return nil, fmt.Errorf("failed to append certificates from PEM (check if hex decodes to valid PEM)")
	}

	// We use empty server name override because we authorize the specific cert in the pool.
	// However, if the cert doesn't have the IP/SAN of the LND host, we might need an override.
	// But usually LND certs have them.
	var creds credentials.TransportCredentials
	if conf.DisableTls {
		// Create a TLS config that skips verification
		tlsConfig := &tls.Config{
			InsecureSkipVerify: true,
		}
		creds = credentials.NewTLS(tlsConfig)
		log.Warn().Msg("LND TLS verification DISABLED via configuration. This is unsafe for production if traffic traverses untrusted networks.")
	} else {
		creds = credentials.NewClientTLSFromCert(cp, "")
	}
	macCred := NewMacaroonCredential(conf.Macaroon)

	// Address of an LND instance
	conn, err := grpc.Dial(
		conf.Address,
		grpc.WithTransportCredentials(creds),
		grpc.WithPerRPCCredentials(macCred),
	)
	if err != nil {
		log.Fatal().Msgf("Failed to connect to LND gRPC: %v", err)
	}

	client := lnrpc.NewLightningClient(conn)
	routerClient := routerrpc.NewRouterClient(conn)
	chainNotifierClient := chainrpc.NewChainNotifierClient(conn)
	return &LndClient{
		client:              client,
		routerClient:        routerClient,
		chainNotifierClient: chainNotifierClient,
		conn:                conn,
		peersubs:            make(map[string]map[uint64]chan struct{}),
		chansubs:            make(map[string]map[uint64]chan struct{}),
	}, nil
}

func (c *LndClient) Close() {
	cancel := c.listenerCancel
	if cancel != nil {
		cancel()
	}
	c.conn.Close()
}

func (c *LndClient) StartListeners() {
	c.listenerCtx, c.listenerCancel = context.WithCancel(context.Background())
	go c.listenPeerEvents()
	go c.listenChannelEvents()
}

func (c *LndClient) listenPeerEvents() {
	ctx := c.listenerCtx
	for {
		if ctx.Err() != nil {
			return
		}

		sub, err := c.client.SubscribePeerEvents(
			ctx,
			&lnrpc.PeerEventSubscription{},
		)
		if err != nil {
			log.Debug().Err(err).Msg("LND: SubscribePeerEvents failed, retrying")
			<-time.After(time.Second)
			continue
		}

		for {
			if ctx.Err() != nil {
				return
			}

			msg, err := sub.Recv()
			if err != nil {
				status, ok := status.FromError(err)
				if ok && status.Code() == codes.Canceled {
					log.Debug().Msg("LND: Peer event listener stopped - context canceled")
					break
				}

				log.Error().Err(err).Msg("LND: Peer event listener error")
				break
			}

			if msg.Type != lnrpc.PeerEvent_PEER_ONLINE {
				continue
			}

			c.submtx.RLock()
			subs, ok := c.peersubs[msg.PubKey]
			if ok {
				for _, sub := range subs {
					sub <- struct{}{}
				}
			}
			c.submtx.RUnlock()
		}

		<-time.After(time.Second)
	}
}

func (c *LndClient) listenChannelEvents() {
	ctx := c.listenerCtx
	for {
		if ctx.Err() != nil {
			return
		}

		sub, err := c.client.SubscribeChannelEvents(
			ctx,
			&lnrpc.ChannelEventSubscription{},
		)
		if err != nil {
			log.Debug().Err(err).Msg("LND: SubscribeChannelEvents failed, retrying")
			<-time.After(time.Second)
			continue
		}

		for {
			if ctx.Err() != nil {
				return
			}

			msg, err := sub.Recv()
			if err != nil {
				status, ok := status.FromError(err)
				if ok && status.Code() == codes.Canceled {
					log.Debug().Msg("LND: Channel event listener stopped - context canceled")
					break
				}

				log.Error().Err(err).Msg("LND: Channel event listener error")
				break
			}

			if msg.Type != lnrpc.ChannelEventUpdate_ACTIVE_CHANNEL {
				continue
			}

			ch := msg.GetActiveChannel()
			point, err := extractChannelPoint(ch)
			if err != nil {
				log.Debug().Err(err).Interface("channel", ch).Msg("LND: Failed to extract channel point")
				continue
			}

			c.submtx.RLock()
			subs, ok := c.chansubs[point]
			if ok {
				for _, sub := range subs {
					sub <- struct{}{}
				}
			}
			c.submtx.RUnlock()
		}

		<-time.After(time.Second)
	}
}

func extractChannelPoint(cp *lnrpc.ChannelPoint) (string, error) {
	str := cp.GetFundingTxidStr()
	if str == "" {
		b := cp.GetFundingTxidBytes()
		h, err := chainhash.NewHash(b)
		if err != nil {
			return "", err
		}
		str = h.String()
	}

	return fmt.Sprintf("%s:%d", str, cp.OutputIndex), nil
}

func (c *LndClient) GetInfo() (*lightning.GetInfoResult, error) {
	info, err := c.client.GetInfo(context.Background(), &lnrpc.GetInfoRequest{})
	if err != nil {
		log.Error().Msgf("LND: client.GetInfo() error: %v", err)
		return nil, err
	}

	return &lightning.GetInfoResult{
		Alias:  info.Alias,
		Pubkey: info.IdentityPubkey,
	}, nil
}

func (c *LndClient) IsConnected(destination []byte) (bool, error) {
	pubkey := hex.EncodeToString(destination)

	resp, err := c.client.ListPeers(context.Background(), &lnrpc.ListPeersRequest{})
	if err != nil {
		log.Error().Msgf("LND: client.ListPeers() error: %v", err)
		return false, fmt.Errorf("LND: client.ListPeers() error: %w", err)
	}

	for _, peer := range resp.Peers {
		if peer.PubKey == pubkey {
			log.Debug().Str("pubkey", pubkey).Msg("LND: Destination peer is online")
			return true, nil
		}
	}

	log.Debug().Str("pubkey", pubkey).Msg("LND: Destination peer is offline")
	return false, nil
}

func (c *LndClient) OpenChannel(req *lightning.OpenChannelRequest) (*wire.OutPoint, error) {
	var baseFee uint64
	var feeRate uint64
	useFees := false
	if req.RoutingPolicy != nil {
		useFees = true
		baseFee = req.RoutingPolicy.BaseMloki
		feeRate = uint64(req.RoutingPolicy.Ppm)
	}
	lnReq := &lnrpc.OpenChannelRequest{
		NodePubkey:         req.Destination,
		LocalFundingAmount: int64(req.CapacityLoki),
		PushSat:            int64(req.PushLoki),
		Private:            !req.AnnounceChannel,
		CommitmentType:     lnrpc.CommitmentType_ANCHORS,
		BaseFee:            baseFee,
		UseBaseFee:         useFees,
		FeeRate:            feeRate,
		UseFeeRate:         useFees,
	}

	if req.ZeroConf {
		lnReq.ZeroConf = true
		lnReq.SpendUnconfirmed = true
	}

	if req.MinConfs != nil {
		minConfs := *req.MinConfs
		lnReq.MinConfs = int32(minConfs)
		if minConfs == 0 {
			lnReq.SpendUnconfirmed = true
		}
	}

	if req.FeeLokiPerVByte != nil {
		lnReq.SatPerVbyte = uint64(math.Ceil(*req.FeeLokiPerVByte))
	} else if req.TargetConf != nil {
		lnReq.TargetConf = int32(*req.TargetConf)
	}

	channelPoint, err := c.client.OpenChannelSync(context.Background(), lnReq)
	if err != nil {
		log.Error().Msgf("LND: client.OpenChannelSync(%x, %v) error: %v", req.Destination, req.CapacityLoki, err)
		return nil, fmt.Errorf("LND: OpenChannel() error: %w", err)
	}

	result, err := lightning.NewOutPoint(channelPoint.GetFundingTxidBytes(), channelPoint.OutputIndex)
	if err != nil {
		log.Error().Msgf("LND: OpenChannel returned invalid outpoint. error: %v", err)
		return nil, err
	}

	// If a routing policy was provided with a TimeLockDelta, we need to update the channel policy
	// since OpenChannelSync does not support setting the TimeLockDelta during creation.
	if req.RoutingPolicy != nil && req.RoutingPolicy.TimeLockDelta > 0 {
		_, err = c.client.UpdateChannelPolicy(context.Background(), &lnrpc.PolicyUpdateRequest{
			Scope: &lnrpc.PolicyUpdateRequest_ChanPoint{
				ChanPoint: channelPoint,
			},
			BaseFeeMsat:   int64(req.RoutingPolicy.BaseMloki),
			FeeRate:       float64(req.RoutingPolicy.Ppm) / 1_000_000,
			TimeLockDelta: req.RoutingPolicy.TimeLockDelta,
		})
		if err != nil {
			log.Error().Msgf("LND: failed to update channel policy with TimeLockDelta %d: %v", req.RoutingPolicy.TimeLockDelta, err)
			// We don't return error here because the channel is already opened.
		}
	}

	return result, nil
}

func (c *LndClient) GetChannel(peerID []byte, channelPoint wire.OutPoint) (*lightning.GetChannelResult, error) {
	r, err := c.client.ListChannels(context.Background(), &lnrpc.ListChannelsRequest{Peer: peerID})
	if err != nil {
		log.Error().Msgf("client.ListChannels(%x) error: %v", peerID, err)
		return nil, err
	}

	channelPointStr := channelPoint.String()

	for _, c := range r.Channels {
		log.Trace().Msgf("LND: getChannel(%x) found channel %v", peerID, c.ChanId)
		if c.ChannelPoint == channelPointStr && c.Active {
			aliasScid, confirmedScid := mapScidsFromChannel(c)
			return &lightning.GetChannelResult{
				AliasScid:        aliasScid,
				ConfirmedScid:    confirmedScid,
				HtlcMinimumMloki: c.LocalConstraints.MinHtlcMsat,
			}, nil
		}
	}
	log.Info().Msgf("No channel found: getChannel(%x)", peerID)
	return nil, fmt.Errorf("no channel found")
}

func (c *LndClient) GetClosedChannels(nodeID string, channelPoints map[string]uint64) (map[string]uint64, error) {
	r := make(map[string]uint64)
	if len(channelPoints) == 0 {
		return r, nil
	}
	waitingCloseChannels, err := c.getWaitingCloseChannels(nodeID)
	if err != nil {
		return nil, err
	}
	wcc := make(map[string]struct{})
	for _, c := range waitingCloseChannels {
		wcc[c.Channel.ChannelPoint] = struct{}{}
	}
	for c, h := range channelPoints {
		if _, ok := wcc[c]; !ok {
			r[c] = h
		}
	}
	return r, nil
}

func (c *LndClient) getWaitingCloseChannels(nodeID string) ([]*lnrpc.PendingChannelsResponse_WaitingCloseChannel, error) {
	pendingResponse, err := c.client.PendingChannels(context.Background(), &lnrpc.PendingChannelsRequest{})
	if err != nil {
		return nil, err
	}
	var waitingCloseChannels []*lnrpc.PendingChannelsResponse_WaitingCloseChannel
	for _, p := range pendingResponse.WaitingCloseChannels {
		if p.Channel.RemoteNodePub == nodeID {
			waitingCloseChannels = append(waitingCloseChannels, p)
		}
	}
	return waitingCloseChannels, nil
}

func (c *LndClient) GetPeerId(scid *lightning.ShortChannelID) ([]byte, error) {
	scidu64 := uint64(*scid)
	resp, err := c.client.ListChannels(context.Background(), &lnrpc.ListChannelsRequest{})
	if err != nil {
		return nil, err
	}

	for _, ch := range resp.Channels {
		if ch.ChanId == scidu64 || ch.ZeroConfConfirmedScid == scidu64 {
			return hex.DecodeString(ch.RemotePubkey)
		}
		for _, alias := range ch.AliasScids {
			if alias == scidu64 {
				return hex.DecodeString(ch.RemotePubkey)
			}
		}
	}

	return nil, nil
}

func (c *LndClient) WaitOnline(peerID []byte, deadline time.Time) error {
	pkStr := hex.EncodeToString(peerID)
	signal := make(chan struct{}, 10)
	defer close(signal)

	c.submtx.Lock()
	subid := c.index
	c.index++
	subs, ok := c.peersubs[pkStr]
	if !ok {
		subs = make(map[uint64]chan struct{})
		c.peersubs[pkStr] = subs
	}

	subs[subid] = signal
	c.submtx.Unlock()

	defer func() {
		c.submtx.Lock()
		subs, ok := c.peersubs[pkStr]
		if ok {
			delete(subs, subid)
			if len(subs) == 0 {
				delete(c.peersubs, pkStr)
			}
		}
		c.submtx.Unlock()
	}()

	connected, err := c.IsConnected(peerID)
	if err != nil {
		return err
	}
	if connected {
		return nil
	}

	select {
	case <-signal:
		return nil
	case <-time.After(time.Until(deadline)):
		return fmt.Errorf("deadline exceeded")
	}
}

func (c *LndClient) WaitChannelActive(peerID []byte, deadline time.Time) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Fetch the channels for this peer
	chans, err := c.client.ListChannels(ctx, &lnrpc.ListChannelsRequest{
		Peer: peerID,
	})
	if err != nil {
		return err
	}
	if len(chans.Channels) == 0 {
		return fmt.Errorf("no channels with peer")
	}

	// Exit now if a channel is already active.
	for _, ch := range chans.Channels {
		if ch.Active {
			return nil
		}
	}

	signal := make(chan struct{}, 10)
	defer close(signal)

	// Subscribe to channel active events from this channel
	c.submtx.Lock()
	for _, ch := range chans.Channels {
		chansignal := make(chan struct{}, 10)
		defer close(chansignal)

		// forward signals from all channels to the signal aggregate
		go func(c chan struct{}) {
			for msg := range c {
				signal <- msg
			}
		}(chansignal)

		outpoint := ch.ChannelPoint
		subid := c.index
		c.index++
		subs, ok := c.chansubs[outpoint]
		if !ok {
			subs = make(map[uint64]chan struct{})
			c.chansubs[outpoint] = subs
		}

		subs[subid] = chansignal
		defer func() {
			c.submtx.Lock()
			subs, ok := c.chansubs[outpoint]
			if ok {
				delete(subs, subid)
				if len(subs) == 0 {
					delete(c.chansubs, outpoint)
				}
			}
			c.submtx.Unlock()
		}()
	}
	c.submtx.Unlock()

	// Fetch the channels for this peer again, so there is no gap between the
	// subscription and the call.
	chans, err = c.client.ListChannels(ctx, &lnrpc.ListChannelsRequest{
		Peer: peerID,
	})
	if err != nil {
		return err
	}

	// Exit now if a channel is already active.
	for _, ch := range chans.Channels {
		if ch.Active {
			return nil
		}
	}

	select {
	case <-signal:
		return nil
	case <-time.After(time.Until(deadline)):
		return fmt.Errorf("deadline exceeded")
	}
}

func (c *LndClient) ListChannels() ([]*lightning.Channel, error) {
	channels, err := c.client.ListChannels(
		context.TODO(),
		&lnrpc.ListChannelsRequest{},
	)
	if err != nil {
		return nil, err
	}
	pendingChannels, err := c.client.PendingChannels(
		context.TODO(),
		&lnrpc.PendingChannelsRequest{},
	)
	if err != nil {
		return nil, err
	}

	result := make([]*lightning.Channel, len(channels.Channels))
	for i, c := range channels.Channels {
		peerId, err := hex.DecodeString(c.RemotePubkey)
		if err != nil {
			log.Debug().Err(err).Msg("LND: hex.DecodeString failed in ListChannels")
			continue
		}
		alias, confirmedScid := mapScidsFromChannel(c)
		outpoint, err := lightning.NewOutPointFromString(c.ChannelPoint)
		if err != nil {
			log.Debug().Err(err).Str("point", c.ChannelPoint).Msg("LND: failed to parse outpoint in ListChannels")
		}

		// Map pending HTLCs
		var pendingHtlcs []lightning.HTLC
		for _, h := range c.PendingHtlcs {
			pendingHtlcs = append(pendingHtlcs, lightning.HTLC{
				Incoming:         h.Incoming,
				AmountMloki:      uint64(h.Amount),
				HashLock:         h.HashLock,
				ExpirationHeight: h.ExpirationHeight,
				HtlcIndex:        h.HtlcIndex,
			})
		}

		result[i] = &lightning.Channel{
			AliasScid:     alias,
			ConfirmedScid: confirmedScid,
			ChannelPoint:  outpoint,
			PeerId:        peerId,
			PendingHtlcs:  pendingHtlcs,
		}
	}

	for _, c := range pendingChannels.PendingOpenChannels {
		peerId, err := hex.DecodeString(c.Channel.RemoteNodePub)
		if err != nil {
			log.Debug().Err(err).Msg("LND: hex.DecodeString failed in ListChannels (active)")
			continue
		}

		outpoint, err := lightning.NewOutPointFromString(c.Channel.ChannelPoint)
		if err != nil {
			log.Debug().Err(err).Str("point", c.Channel.ChannelPoint).Msg("LND: failed to parse outpoint in ListChannels (active)")
		}
		result = append(result, &lightning.Channel{
			AliasScid:     nil,
			ConfirmedScid: nil,
			ChannelPoint:  outpoint,
			PeerId:        peerId,
		})
	}

	return result, nil
}

func (c *LndClient) GetBlockHeight() (uint32, error) {
	info, err := c.client.GetInfo(context.Background(), &lnrpc.GetInfoRequest{})
	if err != nil {
		return 0, err
	}
	return info.BlockHeight, nil
}

func mapScidsFromChannel(c *lnrpc.Channel) (*lightning.ShortChannelID, *lightning.ShortChannelID) {
	var alias *lightning.ShortChannelID
	var confirmedScid *lightning.ShortChannelID
	if c.ZeroConf {
		if c.ZeroConfConfirmedScid != 0 {
			confirmedScid = (*lightning.ShortChannelID)(&c.ZeroConfConfirmedScid)
		}
		alias = (*lightning.ShortChannelID)(&c.ChanId)
	} else {
		confirmedScid = (*lightning.ShortChannelID)(&c.ChanId)
		if len(c.AliasScids) > 0 {
			alias = (*lightning.ShortChannelID)(&c.AliasScids[0])
		}
	}

	return alias, confirmedScid
}

func (c *LndClient) SendCustomMessage(ctx context.Context, msg *lnrpc.SendCustomMessageRequest) (*lnrpc.SendCustomMessageResponse, error) {
	return c.client.SendCustomMessage(ctx, msg)
}

func (c *LndClient) SubscribeCustomMessages(ctx context.Context, req *lnrpc.SubscribeCustomMessagesRequest) (lnrpc.Lightning_SubscribeCustomMessagesClient, error) {
	return c.client.SubscribeCustomMessages(ctx, req)
}

func (c *LndClient) AddInvoice(ctx context.Context, req *lnrpc.Invoice) (*lnrpc.AddInvoiceResponse, error) {
	return c.client.AddInvoice(ctx, req)
}

func (c *LndClient) WalletBalance(ctx context.Context) (*lightning.WalletBalanceResult, error) {
	resp, err := c.client.WalletBalance(ctx, &lnrpc.WalletBalanceRequest{})
	if err != nil {
		return nil, err
	}
	return &lightning.WalletBalanceResult{
		TotalBalance:       resp.TotalBalance,
		ConfirmedBalance:   resp.ConfirmedBalance,
		UnconfirmedBalance: resp.UnconfirmedBalance,
	}, nil
}
