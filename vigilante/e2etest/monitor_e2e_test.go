//go:build e2e
// +build e2e

package e2etest

import (
	"fmt"
	"time"

	ancclient "github.com/anon-org/anon/v4/client/client"
	"github.com/anon-org/vigilante/btcclient"
	"github.com/anon-org/vigilante/metrics"
	"github.com/anon-org/vigilante/monitor"
	"github.com/anon-org/vigilante/reporter"
	"github.com/anon-org/vigilante/submitter"
	"github.com/anon-org/vigilante/testutil"
	"github.com/anon-org/vigilante/types"
	"github.com/btcsuite/btcd/chaincfg"
	sdk "github.com/cosmos/cosmos-sdk/types"
	promtestutil "github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"testing"
)

// TestMonitorBootstrap - validates that after a restart monitor bootstraps from DB
func TestMonitorBootstrap(t *testing.T) {
	t.Parallel()
	numMatureOutputs := uint32(150)

	tm := StartManager(t, WithNumMatureOutputs(numMatureOutputs), WithEpochInterval(2))
	defer tm.Stop(t)

	backend, err := btcclient.NewNodeBackend(
		btcclient.ToBitcoindConfig(tm.Config.BTC),
		&chaincfg.RegressionNetParams,
		&btcclient.EmptyHintCache{},
	)
	require.NoError(t, err)

	err = backend.Start()
	require.NoError(t, err)

	dbBackend := testutil.MakeTestBackend(t)

	monitorMetrics := metrics.NewMonitorMetrics()
	genesisPath := fmt.Sprintf("%s/config/genesis.json", tm.Config.Anon.KeyDirectory)
	genesisInfo, err := types.GetGenesisInfoFromFile(genesisPath, zap.NewNop())
	require.NoError(t, err)

	tm.Config.Submitter.PollingIntervalSeconds = 1
	subAddr, _ := sdk.AccAddressFromBech32(submitterAddrStr)

	// create submitter
	vigilantSubmitter, err := submitter.New(
		&tm.Config.Submitter,
		logger,
		tm.BTCClient,
		tm.AnonClient,
		subAddr,
		tm.Config.Common.RetrySleepTime,
		tm.Config.Common.MaxRetrySleepTime,
		tm.Config.Common.MaxRetryTimes,
		metrics.NewSubmitterMetrics(),
		testutil.MakeTestBackend(t),
		tm.Config.BTC.WalletName,
	)

	require.NoError(t, err)

	vigilantSubmitter.Start()
	defer vigilantSubmitter.Stop()

	vigilantReporter, err := reporter.New(
		&tm.Config.Reporter,
		logger,
		tm.BTCClient,
		tm.AnonClient,
		backend,
		tm.Config.Common.RetrySleepTime,
		tm.Config.Common.MaxRetrySleepTime,
		tm.Config.Common.MaxRetryTimes,
		metrics.NewReporterMetrics(),
	)
	require.NoError(t, err)

	defer func() {
		vigilantSubmitter.Stop()
		vigilantSubmitter.WaitForShutdown()
	}()

	mon, err := monitor.New(
		&tm.Config.Monitor,
		&tm.Config.Common,
		zap.NewNop(),
		genesisInfo,
		tm.AnonClient,
		tm.BTCClient,
		backend,
		monitorMetrics,
		dbBackend,
	)
	require.NoError(t, err)
	vigilantReporter.Start()
	defer vigilantReporter.Stop()

	go func() {
		ticker := time.NewTicker(3 * time.Second)
		timer := time.NewTimer(15 * time.Second)
		defer timer.Stop()
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				tm.mineBlock(t)
			case <-timer.C:
				return
			}
		}
	}()

	go mon.Start(genesisInfo.GetBaseBTCHeight())

	time.Sleep(15 * time.Second)
	mon.Stop()

	// use a new anc client
	anonClient, err := ancclient.New(&tm.Config.Anon, nil)
	require.NoError(t, err)
	defer anonClient.Stop()

	mon, err = monitor.New(
		&tm.Config.Monitor,
		&tm.Config.Common,
		zap.NewNop(),
		genesisInfo,
		anonClient,
		tm.BTCClient,
		backend,
		monitorMetrics,
		dbBackend,
	)
	require.NoError(t, err)
	go mon.Start(genesisInfo.GetBaseBTCHeight())

	defer mon.Stop()

	require.Zero(t, promtestutil.ToFloat64(mon.Metrics().InvalidBTCHeadersCounter))
	require.Zero(t, promtestutil.ToFloat64(mon.Metrics().InvalidEpochsCounter))
	require.Eventually(t, func() bool {
		return mon.BTCScanner.GetBaseHeight() > genesisInfo.GetBaseBTCHeight()
	}, eventuallyWaitTimeOut, eventuallyPollTime)
}
