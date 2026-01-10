package main

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"syscall"

	"github.com/rs/zerolog/log"

	ecies "github.com/ecies/go/v2"
	btcec "github.com/flokiorg/go-flokicoin/crypto"
	"github.com/flowgate-lsp/flspd/build"
	"github.com/flowgate-lsp/flspd/chain"
	"github.com/flowgate-lsp/flspd/common"
	"github.com/flowgate-lsp/flspd/config"
	"github.com/flowgate-lsp/flspd/history"
	"github.com/flowgate-lsp/flspd/interceptor"
	"github.com/flowgate-lsp/flspd/lightning"
	"github.com/flowgate-lsp/flspd/lnd"
	"github.com/flowgate-lsp/flspd/mempool"
	"github.com/flowgate-lsp/flspd/notifications"
	"github.com/flowgate-lsp/flspd/postgresql"
	"github.com/urfave/cli"
)

func runLspd(cliCtx *cli.Context) error {
	common.InitLogger()
	log.Info().Msgf(`Starting flspd, tag='%s', revision='%s'`, build.GetTag(), build.GetRevision())
	n := os.Getenv("NODES")
	var nodeConfigs []*config.NodeConfig
	err := json.Unmarshal([]byte(n), &nodeConfigs)
	if err != nil {
		log.Fatal().Msgf("failed to unmarshal NODES env: %v", err)
	}

	if len(nodeConfigs) == 0 {
		log.Fatal().Msgf("need at least one node configured in NODES.")
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

	ctx, cancel := context.WithCancel(context.Background())
	notificationService := notifications.NewNotificationService(notificationsStore)
	go notificationService.Start(ctx)
	openingService := common.NewOpeningService(openingStore, nodesService)
	notificationCleanupService := notifications.NewCleanupService(notificationsStore)
	go notificationCleanupService.Start(ctx)
	channelSync := history.NewChannelSync(nodes, historyStore)
	go channelSync.ChannelsSynchronize(ctx)

	var interceptors []interceptor.HtlcInterceptor
	for _, node := range nodes {
		var htlcInterceptor interceptor.HtlcInterceptor
		var feeEstimator chain.FeeEstimator
		if mempoolClient == nil {
			feeEstimator = chain.NewDefaultFeeEstimator(node.NodeConfig.TargetConf)
		} else {
			feeEstimator = mempoolClient
		}

		if node.NodeConfig.Lnd == nil {
			log.Fatal().Msgf("node has to be lnd")
		}

		client, err := lnd.NewLndClient(node.NodeConfig.Lnd)
		if err != nil {
			log.Fatal().Msgf("failed to initialize LND client: %v", err)
		}

		client.StartListeners()

		forwardSync := lnd.NewForwardSync(node.NodeId, client, historyStore)
		go forwardSync.ForwardsSynchronize(ctx)
		handler := interceptor.NewInterceptHandler(client, node.NodeConfig, interceptStore, historyStore, openingService, feeEstimator, feeStrategy, notificationService)
		combinedHandler := common.NewCombinedHandler(handler)
		htlcInterceptor, err = lnd.NewLndHtlcInterceptor(node.NodeConfig, client, combinedHandler)
		if err != nil {
			log.Fatal().Msgf("failed to initialize LND interceptor: %v", err)
		}

		interceptors = append(interceptors, htlcInterceptor)
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
		pk, err := hex.DecodeString(config.FlspdPrivateKey)
		if err != nil {
			return nil, fmt.Errorf("hex.DecodeString(config.flspdPrivateKey=%v) error: %v", config.FlspdPrivateKey, err)
		}

		eciesPrivateKey := ecies.NewPrivateKeyFromBytes(pk)
		eciesPublicKey := eciesPrivateKey.PublicKey
		privateKey, publicKey := btcec.PrivKeyFromBytes(pk)
		node := &common.Node{
			NodeConfig:      config,
			PrivateKey:      privateKey,
			PublicKey:       publicKey,
			EciesPrivateKey: eciesPrivateKey,
			EciesPublicKey:  eciesPublicKey,
		}

		if config.Lnd == nil {
			return nil, fmt.Errorf("node has to be lnd")
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
	}

	return nodes, nil
}
