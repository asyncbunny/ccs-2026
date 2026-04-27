package cmd

import (
	"fmt"

	ancclient "github.com/anon-org/anon/v4/client/client"
	"github.com/spf13/cobra"

	"github.com/anon-org/vigilante/btcclient"
	"github.com/anon-org/vigilante/config"
	"github.com/anon-org/vigilante/metrics"
	"github.com/anon-org/vigilante/reporter"
)

// GetReporterCmd returns the CLI commands for the reporter
func GetReporterCmd() *cobra.Command {
	var anonKeyDir string
	var cfgFile = ""

	cmd := &cobra.Command{
		Use:   "reporter",
		Short: "Vigilant reporter",
		Run: func(_ *cobra.Command, _ []string) {
			var (
				err              error
				cfg              config.Config
				btcClient        *btcclient.Client
				anonClient    *ancclient.Client
				vigilantReporter *reporter.Reporter
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

			// apply the flags from CLI
			if len(anonKeyDir) != 0 {
				cfg.Anon.KeyDirectory = anonKeyDir
			}

			// create BTC client and connect to BTC server
			// Note that vigilant reporter needs to subscribe to new BTC blocks
			btcClient, err = btcclient.NewWallet(&cfg, rootLogger)
			if err != nil {
				panic(fmt.Errorf("failed to open BTC client: %w", err))
			}

			// create Anon client. Note that requests from Anon client are ad hoc
			anonClient, err = ancclient.New(&cfg.Anon, nil)
			if err != nil {
				panic(fmt.Errorf("failed to open Anon client: %w", err))
			}

			// register reporter metrics
			reporterMetrics := metrics.NewReporterMetrics()

			// create the chain notifier
			btcNotifier, err := btcclient.NewNodeBackendWithParams(cfg.BTC)
			if err != nil {
				panic(err)
			}

			// create reporter
			vigilantReporter, err = reporter.New(
				&cfg.Reporter,
				rootLogger,
				btcClient,
				anonClient,
				btcNotifier,
				cfg.Common.RetrySleepTime,
				cfg.Common.MaxRetrySleepTime,
				cfg.Common.MaxRetryTimes,
				reporterMetrics,
			)
			if err != nil {
				panic(fmt.Errorf("failed to create vigilante reporter: %w", err))
			}

			// start Prometheus metrics server
			addr := fmt.Sprintf("%s:%d", cfg.Metrics.Host, cfg.Metrics.ServerPort)
			metrics.Start(addr, reporterMetrics.Registry)

			// start normal-case execution
			vigilantReporter.Start()

			addInterruptHandler(func() {
				rootLogger.Info("Stopping reporter...")
				vigilantReporter.Stop()
				rootLogger.Info("Reporter shutdown")
			})
			addInterruptHandler(func() {
				rootLogger.Info("Stopping BTC client...")
				btcClient.Stop()
				btcClient.WaitForShutdown()
				rootLogger.Info("BTC client shutdown")
			})

			<-interruptHandlersDone
			rootLogger.Info("Shutdown complete")
		},
	}
	cmd.Flags().StringVar(&anonKeyDir, "anon-key-dir", "", "Directory of the Anon key")
	cmd.Flags().StringVar(&cfgFile, "config", config.DefaultConfigFile(), "config file")

	return cmd
}
