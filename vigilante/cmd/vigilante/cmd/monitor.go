package cmd

import (
	"fmt"

	ancqccfg "github.com/anon-org/anon/v4/client/config"
	ancqc "github.com/anon-org/anon/v4/client/query"
	"github.com/spf13/cobra"

	"github.com/anon-org/vigilante/btcclient"
	"github.com/anon-org/vigilante/config"
	"github.com/anon-org/vigilante/metrics"
	"github.com/anon-org/vigilante/monitor"
	"github.com/anon-org/vigilante/types"
)

const (
	genesisFileNameFlag    = "genesis"
	GenesisFileNameDefault = "genesis.json"
)

// GetMonitorCmd returns the CLI commands for the monitor
func GetMonitorCmd() *cobra.Command {
	var genesisFile string
	var cfgFile = ""
	// Group monitor queries under a subcommand
	cmd := &cobra.Command{
		Use:   "monitor",
		Short: "Vigilante monitor constantly checks the consistency between the Anon node and BTC and detects censorship of BTC checkpoints",
		Run: func(_ *cobra.Command, _ []string) {
			var (
				err              error
				cfg              config.Config
				btcClient        *btcclient.Client
				ancQueryClient   *ancqc.QueryClient
				vigilanteMonitor *monitor.Monitor
			)

			// get the config from the given file or the default file
			cfg, err = config.New(cfgFile)
			if err != nil {
				panic(fmt.Errorf("failed to load config: %w", err))
			}
			rootLogger, err := cfg.CreateLogger()
			if err != nil {
				panic(fmt.Errorf("failed to create logger: %w", err))
			}

			// create Anon query client. Note that requests from Anon client are ad hoc
			queryCfg := &ancqccfg.AnonQueryConfig{
				RPCAddr: cfg.Anon.RPCAddr,
				Timeout: cfg.Anon.Timeout,
			}
			if err := queryCfg.Validate(); err != nil {
				panic(fmt.Errorf("invalid config for query client: %w", err))
			}
			ancQueryClient, err = ancqc.New(queryCfg)
			if err != nil {
				panic(fmt.Errorf("failed to create anon query client: %w", err))
			}

			// create BTC client and connect to BTC server
			btcClient, err = btcclient.NewWallet(&cfg, rootLogger)
			if err != nil {
				panic(fmt.Errorf("failed to open BTC client: %w", err))
			}
			genesisInfo, err := types.GetGenesisInfoFromFile(genesisFile, rootLogger)
			if err != nil {
				panic(fmt.Errorf("failed to read genesis file: %w", err))
			}

			// register monitor metrics
			monitorMetrics := metrics.NewMonitorMetrics()

			// create the chain notifier
			btcNotifier, err := btcclient.NewNodeBackendWithParams(cfg.BTC)
			if err != nil {
				panic(err)
			}

			dbBackend, err := cfg.Monitor.DatabaseConfig.GetDBBackend()
			if err != nil {
				panic(err)
			}

			// create monitor
			vigilanteMonitor, err = monitor.New(
				&cfg.Monitor,
				&cfg.Common,
				rootLogger,
				genesisInfo,
				ancQueryClient,
				btcClient,
				btcNotifier,
				monitorMetrics,
				dbBackend,
			)
			if err != nil {
				panic(fmt.Errorf("failed to create vigilante monitor: %w", err))
			}

			// start
			go vigilanteMonitor.Start(genesisInfo.GetBaseBTCHeight())

			// start Prometheus metrics server
			addr := fmt.Sprintf("%s:%d", cfg.Metrics.Host, cfg.Metrics.ServerPort)
			metrics.Start(addr, monitorMetrics.Registry)

			addInterruptHandler(func() {
				rootLogger.Info("Stopping monitor...")
				vigilanteMonitor.Stop()
				rootLogger.Info("Monitor shutdown")
			})

			<-interruptHandlersDone
			rootLogger.Info("Shutdown complete")
		},
	}
	cmd.Flags().StringVar(&genesisFile, genesisFileNameFlag, GenesisFileNameDefault, "genesis file")
	cmd.Flags().StringVar(&cfgFile, "config", config.DefaultConfigFile(), "config file")

	return cmd
}
