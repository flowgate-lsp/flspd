package main

import (
	"context"
	"encoding/hex"
	"fmt"

	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/flokiorg/flnd/funding"
	"github.com/nbd-wtf/go-nostr"
	"github.com/rs/zerolog/log"
	"gopkg.in/yaml.v3"

	ecies "github.com/ecies/go/v2"
	btcec "github.com/flokiorg/go-flokicoin/crypto"
	"github.com/flowgate-lsp/flspd/build"
	"github.com/flowgate-lsp/flspd/chain"
	"github.com/flowgate-lsp/flspd/common"
	"github.com/flowgate-lsp/flspd/config"
	"github.com/flowgate-lsp/flspd/dashboard"
	"github.com/flowgate-lsp/flspd/history"
	"github.com/flowgate-lsp/flspd/interceptor"
	"github.com/flowgate-lsp/flspd/lightning"
	"github.com/flowgate-lsp/flspd/lnd"
	"github.com/flowgate-lsp/flspd/lsps"
	"github.com/flowgate-lsp/flspd/mempool"
	"github.com/flowgate-lsp/flspd/notifications"
	"github.com/flowgate-lsp/flspd/postgresql"
	"github.com/urfave/cli"
)

// Default configuration values
const (
	DefaultMaxPaymentSizeLoki    = 10_000_000_000 // 10k Loki
	DefaultMinLifetimeBlocks     = 43200          // ~1 month (1 minute blocks)
	DefaultMaxClientToSelfDelay  = 10080          // ~1 week
	DefaultFeeParamsValidity     = time.Hour      // Fee params validity duration
	DefaultLsps1MaxPendingOrders = 50             // Max pending LSPS1 orders per peer
	DefaultLsps5MaxWebhooks      = 100            // Max LSPS5 webhooks per client

	// LSP Policy Limits
	AbsoluteMinChannelCapacityLoki = 20000 // Hard floor for channel size enforced by LSP
)

func runLspd(cliCtx *cli.Context) error {
	// Sanity Checks: Ensure LSP policy complies with Network limits

	// LSP Min Capacity must be >= Network Min Funding
	if AbsoluteMinChannelCapacityLoki < uint64(funding.MinChanFundingSize) {
		return fmt.Errorf("configuration error: AbsoluteMinChannelCapacityLoki (%d) is less than network limit (%d)", AbsoluteMinChannelCapacityLoki, funding.MinChanFundingSize)
	}

	// LSP Min Capacity must be <= Network Max Funding (otherwise we can never open a standard channel)
	if AbsoluteMinChannelCapacityLoki > uint64(funding.MaxFlcFundingAmount) {
		return fmt.Errorf("configuration error: AbsoluteMinChannelCapacityLoki (%d) exceeds standard maximum funding amount (%d)", AbsoluteMinChannelCapacityLoki, funding.MaxFlcFundingAmount)
	}

	// Network Min Funding must be <= Network Max Funding (Basic sanity of external package)
	if funding.MinChanFundingSize > funding.MaxFlcFundingAmount {
		return fmt.Errorf("external consistency error: Network MinChanFundingSize (%d) > MaxFlcFundingAmount (%d)", funding.MinChanFundingSize, funding.MaxFlcFundingAmount)
	}
	common.InitLogger()
	log.Info().Msgf(`Starting flspd, tag='%s', revision='%s'`, build.GetTag(), build.GetRevision())
	// Load nodes configuration
	var nodesContent []byte
	var err error

	if nodesJson := os.Getenv("NODES_JSON"); nodesJson != "" {
		nodesContent = []byte(nodesJson)
	} else {
		nodesConfigPath := os.Getenv("NODES_CONFIG_PATH")
		if nodesConfigPath == "" {
			return fmt.Errorf("NODES_CONFIG_PATH environment variable is not set")
		}

		nodesContent, err = os.ReadFile(nodesConfigPath)
		if err != nil {
			return fmt.Errorf("failed to read nodes config file: %w", err)
		}
	}

	var nodeConfigs []*config.NodeConfig
	err = yaml.Unmarshal(nodesContent, &nodeConfigs)
	if err != nil {
		log.Fatal().Msgf("failed to unmarshal nodes config from file: %v", err)
	}

	if len(nodeConfigs) == 0 {
		log.Fatal().Msgf("need at least one node configured in nodes config file.")
	}

	nodes, err := initializeNodes(nodeConfigs)
	if err != nil {
		log.Fatal().Msgf("failed to initialize nodes: %v", err)
	}

	nodesService, err := common.NewNodesService(nodes)
	if err != nil {
		log.Fatal().Msgf("failed to create nodes service: %v", err)
	}

	var feeStrategy chain.FeeStrategy
	envFeeStrategy := os.Getenv("MEMPOOL_PRIORITY")
	switch strings.ToLower(envFeeStrategy) {
	case "minimum":
		feeStrategy = chain.FeeStrategyMinimum
	case "economy":
		feeStrategy = chain.FeeStrategyEconomy
	case "hour":
		feeStrategy = chain.FeeStrategyHour
	case "halfhour":
		feeStrategy = chain.FeeStrategyHalfHour
	case "fastest":
		feeStrategy = chain.FeeStrategyFastest
	default:
		feeStrategy = chain.FeeStrategyEconomy
	}

	var mempoolClient *mempool.MempoolClient
	mempoolUrl := os.Getenv("MEMPOOL_API_BASE_URL")
	if mempoolUrl != "" {
		mempoolClient, err = mempool.NewMempoolClient(mempoolUrl)
		if err != nil {
			log.Fatal().Msgf("failed to initialize mempool client: %v", err)
		}

		log.Info().Msgf("using mempool api for fee estimation: %v, fee strategy: %v:%v", mempoolUrl, envFeeStrategy, feeStrategy)
	}

	databaseUrl := os.Getenv("DATABASE_URL")
	automigrate, _ := strconv.ParseBool(os.Getenv("AUTO_MIGRATE_DATABASE"))
	if automigrate {
		err = postgresql.Migrate(databaseUrl)
		if err != nil {
			log.Fatal().Msgf("Failed to migrate postgres database: %v", err)
		}
	}
	pool, err := postgresql.PgConnect(databaseUrl)
	if err != nil {
		log.Fatal().Msgf("pgConnect() error: %v", err)
	}

	interceptStore := postgresql.NewPostgresInterceptStore(pool)
	openingStore := postgresql.NewPostgresOpeningStore(pool)
	notificationsStore := postgresql.NewNotificationsStore(pool)
	historyStore := postgresql.NewHistoryStore(pool)

	// Logic: We no longer auto-adjust values at runtime. Instead, we validated
	// them above to ensure they are safe based on the configured padding.
	for _, node := range nodes {
		for _, token := range node.Tokens {
			minPaymentSize := node.NodeConfig.MinPaymentSizeMloki
			maxPaymentSize := node.NodeConfig.JitMaxPaymentSizeMloki
			if maxPaymentSize == 0 {
				maxPaymentSize = DefaultMaxPaymentSizeLoki
			}

			// Proportional from ChannelFeePermyriad (in basis points of basis points)
			// ChannelFeePermyriad is per 10,000. PPM is per 1,000,000. So multiply by 100.
			proportional := uint32(node.NodeConfig.ChannelFeePermyriad * 100)

			params := common.OpeningFeeParams{
				MinFeeMloki:          uint64(node.NodeConfig.ChannelMinimumFeeMloki),
				Proportional:         proportional,
				ValidUntil:           "",
				MinLifetime:          DefaultMinLifetimeBlocks,
				MaxClientToSelfDelay: DefaultMaxClientToSelfDelay,
				MinPaymentSizeMloki:  minPaymentSize,
				MaxPaymentSizeMloki:  maxPaymentSize,
				Promise:              "sync_from_config",
			}

			// Upsert for 1 hour validity
			err := openingStore.SetFeeParams(token, DefaultFeeParamsValidity, params)
			if err != nil {
				log.Error().Msgf("Failed to sync fee params for token %s: %v", token, err)
			} else {
				log.Info().Msgf("Synced fee params for token %s: min_size=%d, max_size=%d", token, minPaymentSize, maxPaymentSize)
			}
		}
	}

	ctx, cancel := context.WithCancel(context.Background())
	notificationService := notifications.NewNotificationService(openingStore, nodesService, notificationsStore)

	// Configure Nostr transport if private key is provided
	lspNostrPrivKey := os.Getenv("LSP_NOSTR_PRIVATE_KEY")
	if lspNostrPrivKey != "" {
		var err error
		lspNostrPrivKey, err = common.ParsePrivateKey(lspNostrPrivKey)
		if err != nil {
			log.Fatal().Err(err).Msg("Invalid LSP_NOSTR_PRIVATE_KEY")
		}

		nostrTransport := notifications.NewNostrTransport(lspNostrPrivKey)
		notificationService.AddTransport(nostrTransport)
		log.Info().Msg("Nostr notification transport enabled")
	}

	go notificationService.Start(ctx)
	openingService := common.NewOpeningService(openingStore, nodesService)

	// Initialize Dashboard Service
	dashboardAdminPubkey := os.Getenv("DASHBOARD_ADMIN_PUBKEY")
	dashboardListenAddr := os.Getenv("DASHBOARD_LISTEN_ADDRESS")
	if dashboardListenAddr == "" {
		dashboardListenAddr = ":8080"
	}

	dashSvc := dashboard.NewDashboardService(
		nodesService,
		openingService,
		openingStore,
		historyStore,
		dashboardAdminPubkey,
		os.Getenv("NODES_CONFIG_PATH"),
	)

	// Start dashboard HTTP server
	go func() {
		log.Info().Msgf("Starting dashboard server on %s", dashboardListenAddr)
		if err := http.ListenAndServe(dashboardListenAddr, dashSvc); err != nil {
			log.Error().Err(err).Msg("Dashboard server failed")
		}
	}()

	// Use bridge for notifications to update dashboard in real-time
	dashBridge := NewDashboardNotificationBridge(notificationService, dashSvc)
	notificationCleanupService := notifications.NewCleanupService(notificationsStore)
	go notificationCleanupService.Start(ctx)
	orderCleanupService := lsps.NewOrderCleanupService(openingStore, 24*time.Hour)
	go orderCleanupService.Start()
	channelSync := history.NewChannelSync(nodes, historyStore)
	go channelSync.ChannelsSynchronize(ctx)

	var settlementServices []*lsps.Lsps1SettlementService
	var interceptors []interceptor.HtlcInterceptor
	var lspsServers []*lsps.Server
	for _, node := range nodes {
		var htlcInterceptor interceptor.HtlcInterceptor
		var feeEstimator chain.FeeEstimator
		if mempoolClient == nil {
			feeEstimator = chain.NewDefaultFeeEstimator(node.NodeConfig.TargetConf)
		} else {
			feeEstimator = mempoolClient
		}

		if node.NodeConfig.Lnd == nil {
			log.Fatal().Msgf("node has to be flnd")
		}

		client, err := lnd.NewLndClient(node.NodeConfig.Lnd)
		if err != nil {
			log.Fatal().Msgf("failed to initialize LND client: %v", err)
		}

		client.StartListeners()

		// Start LSPS1 settlement service for this node
		settlementSvc := lsps.NewLsps1SettlementService(client, openingStore, historyStore, node.NodeConfig, feeEstimator, feeStrategy, dashBridge)
		err = settlementSvc.Start(ctx)
		if err != nil {
			log.Error().Err(err).Msg("Failed to start LSPS1 settlement service")
		} else {
			settlementServices = append(settlementServices, settlementSvc)
		}

		forwardSync := lnd.NewForwardSync(node.NodeId, client, historyStore)
		go forwardSync.ForwardsSynchronize(ctx)
		notificationService.SetNodePrivKey(node.PrivateKey)
		handler := interceptor.NewInterceptHandler(client, node.NodeConfig, interceptStore, historyStore, openingService, feeEstimator, feeStrategy, dashBridge, node.PrivateKey)
		combinedHandler := common.NewCombinedHandler(handler)
		htlcInterceptor, err = lnd.NewLndHtlcInterceptor(node.NodeConfig, client, combinedHandler)
		if err != nil {
			log.Fatal().Msgf("failed to initialize LND interceptor: %v", err)
		}

		interceptors = append(interceptors, htlcInterceptor)

		lsps1MaxOrders, _ := strconv.Atoi(os.Getenv("LSPS1_MAX_PENDING_ORDERS"))
		if lsps1MaxOrders <= 0 {
			lsps1MaxOrders = DefaultLsps1MaxPendingOrders
		}
		lsps5MaxWebhooks, _ := strconv.Atoi(os.Getenv("LSPS5_MAX_WEBHOOKS"))
		if lsps5MaxWebhooks <= 0 {
			lsps5MaxWebhooks = DefaultLsps5MaxWebhooks
		}

		var lspNostrPubkey string
		if lspNostrPrivKey != "" {
			var err error
			lspNostrPubkey, err = nostr.GetPublicKey(lspNostrPrivKey)
			if err != nil {
				log.Error().Err(err).Msg("Failed to derive Nostr public key")
			}
		}

		lspsConfig := lsps.Config{
			Lsps1MaxPendingOrders: lsps1MaxOrders,
			Lsps5MaxWebhooks:      lsps5MaxWebhooks,
			EnableLsps5Https:      true,
			EnableLsps5Nostr:      lspNostrPrivKey != "",
			Lsps5NostrPubkey:      lspNostrPubkey,
		}

		liquidityManager := lsps.NewLiquidityManager(client, openingStore)
		lspsServer := lsps.NewServer(lspsConfig, client, nodesService, openingService, liquidityManager, dashBridge)
		lspsServer.Start()
		lspsServers = append(lspsServers, lspsServer)
	}

	address := os.Getenv("LISTEN_ADDRESS")
	certMagicDomain := os.Getenv("CERTMAGIC_DOMAIN")
	cs := NewChannelOpenerServer(interceptStore, openingService)
	ns := notifications.NewNotificationsServer(notificationsStore)
	s, err := NewGrpcServer(nodesService, address, certMagicDomain, cs, ns)
	if err != nil {
		log.Fatal().Msgf("failed to initialize grpc server: %v", err)
	}

	var wg sync.WaitGroup
	wg.Add(len(interceptors) + 1)

	stopInterceptors := func() {
		for _, interceptor := range interceptors {
			interceptor.Stop()
		}

		for _, lspsServer := range lspsServers {
			lspsServer.Stop()
		}

		for _, settlementSvc := range settlementServices {
			settlementSvc.Stop()
		}

		orderCleanupService.Stop()

		cancel()
	}

	for _, interceptor := range interceptors {
		i := interceptor
		go func() {
			err := i.Start()
			if err == nil {
				log.Info().Msgf("Interceptor stopped.")

			} else {
				log.Error().Msgf("FATAL. Interceptor stopped with error: %v", err)
			}

			wg.Done()

			// If any interceptor stops, stop everything, so we're able to restart using systemd.
			s.Stop()
			stopInterceptors()
		}()
	}

	go func() {
		err := s.Start()
		if err == nil {
			log.Info().Msgf("GRPC server stopped.")
		} else {
			log.Error().Msgf("FATAL. GRPC server stopped with error: %v", err)
		}

		wg.Done()

		// If the server stops, stop everything else, so we're able to restart using systemd.
		stopInterceptors()
	}()

	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		sig := <-c
		log.Info().Msgf("Received stop signal %v. Stopping.", sig)

		// Stop everything gracefully on stop signal
		s.Stop()
		stopInterceptors()
	}()

	wg.Wait()
	log.Info().Msgf("flspd exited")
	return nil
}

func initializeNodes(configs []*config.NodeConfig) ([]*common.Node, error) {
	if len(configs) == 0 {
		return nil, fmt.Errorf("no nodes supplied")
	}

	nodes := []*common.Node{}
	for _, config := range configs {
		hexKey, err := common.ParsePrivateKey(config.FlspdPrivateKey)
		if err != nil {
			return nil, fmt.Errorf("ParsePrivateKey(config.flspdPrivateKey) error: %v", err)
		}

		pk, err := hex.DecodeString(hexKey)
		if err != nil {
			return nil, fmt.Errorf("hex.DecodeString error: %v", err)
		}

		eciesPrivateKey := ecies.NewPrivateKeyFromBytes(pk)
		eciesPublicKey := eciesPrivateKey.PublicKey
		privateKey, publicKey := btcec.PrivKeyFromBytes(pk)

		// Validate config using the config package validation logic
		if err := config.Validate(); err != nil {
			return nil, err
		}

		// Validate wumbo limits (PublicChannelAmount or explicit wumbo needs)
		if config.PublicChannelAmount > int64(funding.MaxFlcFundingAmountWumbo) {
			return nil, fmt.Errorf("publicChannelAmount %d exceeds MaxFlcFundingAmountWumbo %d", config.PublicChannelAmount, funding.MaxFlcFundingAmountWumbo)
		}
		// Warn if exceeding standard limit but under wumbo
		if config.PublicChannelAmount > int64(funding.MaxFlcFundingAmount) {
			log.Warn().Msgf("PublicChannelAmount %d exceeds standard MaxFlcFundingAmount %d. Ensure wumbo is enabled.", config.PublicChannelAmount, funding.MaxFlcFundingAmount)
		}

		if err := ValidateLsps1Config(config); err != nil {
			return nil, fmt.Errorf("LSPS1 config validation failed: %w", err)
		}

		if err := ValidateJitConfig(config); err != nil {
			return nil, fmt.Errorf("JIT config validation failed: %w", err)
		}

		node := &common.Node{
			NodeConfig:      config,
			PrivateKey:      privateKey,
			PublicKey:       publicKey,
			EciesPrivateKey: eciesPrivateKey,
			EciesPublicKey:  eciesPublicKey,
		}

		if config.Lnd == nil {
			return nil, fmt.Errorf("node has to be flnd")
		}

		node.Client, err = lnd.NewLndClient(config.Lnd)
		if err != nil {
			return nil, err
		}

		// Make sure the nodes is available and set name and pubkey if not set
		// in config.
		info, err := node.Client.GetInfo()
		if err != nil {
			decodedPubkey, decodeErr := hex.DecodeString(
				node.NodeConfig.NodePubkey)
			if node.NodeConfig.Name == "" || node.NodeConfig.NodePubkey == "" ||
				decodeErr != nil || len(decodedPubkey) != 33 {

				return nil, fmt.Errorf("failed to get info from host %s",
					node.NodeConfig.Host)
			}

			info = &lightning.GetInfoResult{
				Alias:  node.NodeConfig.Name,
				Pubkey: node.NodeConfig.NodePubkey,
			}
		}

		if node.NodeConfig.Name == "" {
			node.NodeConfig.Name = info.Alias
		}

		if node.NodeConfig.NodePubkey == "" {
			node.NodeConfig.NodePubkey = info.Pubkey
		}

		nodeId, err := hex.DecodeString(info.Pubkey)
		if err != nil {
			return nil, fmt.Errorf("failed to parse node id %s", info.Pubkey)
		}

		node.NodeId = nodeId
		node.Tokens = config.Tokens
		nodes = append(nodes, node)

		log.Info().Msgf("LSPS Connection URI: %s@%s", info.Pubkey, config.Host)
	}

	return nodes, nil
}
