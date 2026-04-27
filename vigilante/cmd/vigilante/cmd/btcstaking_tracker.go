package cmd

import (
	"fmt"

	ancclient "github.com/anon-org/anon/v4/client/client"
	"github.com/anon-org/vigilante/btcclient"
	bst "github.com/anon-org/vigilante/btcstaking-tracker"
	"github.com/anon-org/vigilante/config"
	"github.com/anon-org/vigilante/metrics"
	"github.com/spf13/cobra"
)

func GetBTCStakingTracker() *cobra.Command {
	var anonKeyDir string
	var cfgFile = ""
	var startHeight uint64

	cmd := &cobra.Command{
		Use:   "bstracker",
		Short: "BTC staking tracker",
		Run: func(_ *cobra.Command, _ []string) {
			var (
				err error
				cfg config.Config
			)

			// get the config from the given file or the default file
			cfg, err = config.New(cfgFile)
			if err != nil {
				panic(fmt.Errorf("failed to load config: %w", err))
			}
			// apply the flags from CLI
			if len(anonKeyDir) != 0 {
				cfg.Anon.KeyDirectory = anonKeyDir
			}

			rootLogger, err := cfg.CreateLogger()
			if err != nil {
				panic(fmt.Errorf("failed to create logger: %w", err))
			}

			// apply the flags from CLI
			if len(anonKeyDir) != 0 {
				cfg.Anon.KeyDirectory = anonKeyDir
			}

			// create Anon client. Note that requests from Anon client are ad hoc
			ancClient, err := ancclient.New(&cfg.Anon, nil)
			if err != nil {
				panic(fmt.Errorf("failed to open Anon client: %w", err))
			}

			// start Anon client so that WebSocket subscriber can work
			if err := ancClient.Start(); err != nil {
				panic(fmt.Errorf("failed to start WebSocket connection with Anon: %w", err))
			}

			// create BTC client and connect to BTC server
			// Note that monitor needs to subscribe to new BTC blocks
			btcClient, err := btcclient.NewWallet(&cfg, rootLogger)
			if err != nil {
				panic(fmt.Errorf("failed to open BTC client: %w", err))
			}

			// create BTC notifier
			// TODO: is it possible to merge BTC client and BTC notifier?
			btcNotifier, err := btcclient.NewNodeBackendWithParams(cfg.BTC)
			if err != nil {
				panic(err)
			}

			bsMetrics := metrics.NewBTCStakingTrackerMetrics()

			dbBackend, err := cfg.BTCStakingTracker.SlasherDatabaseConfig.GetDBBackend()
			if err != nil {
				panic(err)
			}

			bstracker := bst.NewBTCStakingTracker(
				btcClient,
				btcNotifier,
				ancClient,
				&cfg.BTCStakingTracker,
				&cfg.Common,
				rootLogger,
				bsMetrics,
				dbBackend,
			)

			if err := btcNotifier.Start(); err != nil {
				panic(fmt.Errorf("failed to start btc chain notifier: %w", err))
			}

			// bootstrap
			if err := bstracker.Bootstrap(startHeight); err != nil {
				panic(err)
			}

			err = bstracker.Start()

			if err != nil {
				panic(fmt.Errorf("failed to start unbonding watcher: %w", err))
			}

			// start Prometheus metrics server
			addr := fmt.Sprintf("%s:%d", cfg.Metrics.Host, cfg.Metrics.ServerPort)
			metrics.Start(addr, bsMetrics.Registry)

			// SIGINT handling stuff
			addInterruptHandler(func() {
				rootLogger.Info("Stopping unbonding watcher...")
				if err := bstracker.Stop(); err != nil {
					panic(fmt.Errorf("failed to stop unbonding watcher: %w", err))
				}
				rootLogger.Info("Unbonding watcher shutdown")
			})
			addInterruptHandler(func() {
				rootLogger.Info("Stopping BTC notifier...")
				if err := bstracker.Stop(); err != nil {
					panic(fmt.Errorf("failed to stop btc chain notifier: %w", err))
				}
				rootLogger.Info("BTC notifier shutdown")
			})
			addInterruptHandler(func() {
				rootLogger.Info("Stopping Anon client...")
				if err := ancClient.Stop(); err != nil {
					panic(fmt.Errorf("failed to stop Anon client: %w", err))
				}
				rootLogger.Info("Anon client shutdown")
			})

			<-interruptHandlersDone
			rootLogger.Info("Shutdown complete")
		},
	}
	cmd.Flags().StringVar(&anonKeyDir, "anon-key", "", "Directory of the Anon key")
	cmd.Flags().StringVar(&cfgFile, "config", config.DefaultConfigFile(), "config file")
	cmd.Flags().Uint64Var(&startHeight, "start-height", 0, "height that the BTC slasher starts scanning for evidences")

	return cmd
}
