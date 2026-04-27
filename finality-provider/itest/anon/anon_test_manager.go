//go:build e2e_anon

package e2etest_anon

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/avast/retry-go/v4"
	fpstore "github.com/anon-org/finality-provider/finality-provider/store"
	"github.com/anon-org/finality-provider/metrics"

	ancclient "github.com/anon-org/anon/v4/client/client"

	ccapi "github.com/anon-org/finality-provider/clientcontroller/api"

	"github.com/anon-org/anon/v4/testutil/datagen"
	anctypes "github.com/anon-org/anon/v4/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	"github.com/ory/dockertest/v3"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	fpcc "github.com/anon-org/finality-provider/clientcontroller"
	anccc "github.com/anon-org/finality-provider/clientcontroller/anon"
	"github.com/anon-org/finality-provider/eotsmanager/client"
	eotsconfig "github.com/anon-org/finality-provider/eotsmanager/config"
	fpcfg "github.com/anon-org/finality-provider/finality-provider/config"
	"github.com/anon-org/finality-provider/finality-provider/service"
	e2eutils "github.com/anon-org/finality-provider/itest"
	"github.com/anon-org/finality-provider/itest/container"
	base_test_manager "github.com/anon-org/finality-provider/itest/test-manager"
	"github.com/anon-org/finality-provider/testutil"
	"github.com/anon-org/finality-provider/types"
)

const (
	eventuallyWaitTimeOut = 5 * time.Minute
	eventuallyPollTime    = 1 * time.Second

	testMoniker = "test-moniker"
	testChainID = "chain-test"
	passphrase  = "testpass"
	hdPath      = ""
)

type TestManager struct {
	*base_test_manager.BaseTestManager
	EOTSServerHandler *e2eutils.EOTSServerHandler
	EOTSHomeDir       string
	FpConfig          *fpcfg.Config
	Fps               []*service.FinalityProviderApp
	EOTSClient        *client.EOTSManagerGRPCClient
	ANCConsumerClient *anccc.AnonConsumerController
	baseDir           string
	manager           *container.Manager
	logger            *zap.Logger
	anond          *dockertest.Resource
}

func StartManager(t *testing.T, ctx context.Context, eotsHmacKey string, fpHmacKey string) *TestManager {
	testDir, err := base_test_manager.TempDir(t, "fp-e2e-test-*")
	require.NoError(t, err)

	loggerConfig := zap.NewDevelopmentConfig()
	loggerConfig.Level = zap.NewAtomicLevelAt(zap.ErrorLevel)
	logger, err := loggerConfig.Build()
	require.NoError(t, err)

	// 1. generate covenant committee
	covenantQuorum := 2
	numCovenants := 3
	covenantPrivKeys, covenantPubKeys := e2eutils.GenerateCovenantCommittee(numCovenants, t)

	// 2. prepare Anon node
	manager, err := container.NewManager(t)
	require.NoError(t, err)

	// Create temp dir for anon node
	anonDir, err := base_test_manager.TempDir(t, "anon-test-*")
	require.NoError(t, err)

	// Start anon node in docker
	anond, err := manager.RunAnondResource(t, anonDir, covenantQuorum, covenantPubKeys)
	require.NoError(t, err)
	require.NotNil(t, anond)

	keyDir := filepath.Join(anonDir, "node0", "anond")
	fpHomeDir := filepath.Join(testDir, "fp-home")
	cfg := e2eutils.DefaultFpConfig(keyDir, fpHomeDir)

	// update ports with the dynamically allocated ones from docker
	cfg.AnonConfig.RPCAddr = fmt.Sprintf("http://localhost:%s", anond.GetPort("26657/tcp"))
	cfg.AnonConfig.GRPCAddr = fmt.Sprintf("https://localhost:%s", anond.GetPort("9090/tcp"))

	var bc ccapi.AnonController
	var bcc ccapi.ConsumerController

	// Increase timeout and polling interval for CI environments
	startTimeout := 30 * time.Second
	startPollInterval := 1 * time.Second

	require.Eventually(t, func() bool {
		ancCfg := cfg.AnonConfig.ToAnonConfig()
		ancCl, err := ancclient.New(&ancCfg, logger)
		if err != nil {
			t.Logf("failed to create Anon client: %v", err)
			// Add small delay to avoid overwhelming the system
			time.Sleep(100 * time.Millisecond)
			return false
		}
		bc, err = anccc.NewAnonController(ancCl, cfg.AnonConfig, logger)
		if err != nil {
			t.Logf("failed to create Anon controller: %v", err)
			time.Sleep(100 * time.Millisecond)
			return false
		}

		err = bc.Start()
		if err != nil {
			t.Logf("failed to start Anon controller: %v", err)
			time.Sleep(200 * time.Millisecond)
			return false
		}
		bcc, err = anccc.NewAnonConsumerController(cfg.AnonConfig, logger)
		if err != nil {
			t.Logf("failed to create Anon consumer controller: %v", err)
			time.Sleep(100 * time.Millisecond)
			return false
		}
		return true
	}, startTimeout, startPollInterval)

	// Prepare EOTS manager
	eotsHomeDir := filepath.Join(testDir, "eots-home")
	eotsCfg := eotsconfig.DefaultConfigWithHomePath(eotsHomeDir)
	eotsCfg.RPCListener = fmt.Sprintf("127.0.0.1:%d", testutil.AllocateUniquePort(t))
	eotsCfg.Metrics.Port = testutil.AllocateUniquePort(t)

	disableUnsafeEndpoints := false
	eotsCfg.DisableUnsafeEndpoints = &disableUnsafeEndpoints

	// Set HMAC key for EOTS server if provided
	if eotsHmacKey != "" {
		eotsCfg.HMACKey = eotsHmacKey
		t.Logf("Using EOTS server HMAC key: %s", eotsHmacKey)
	}

	// Set HMAC key for finality provider client if provided
	if fpHmacKey != "" {
		cfg.HMACKey = fpHmacKey
		t.Logf("Using FP client HMAC key: %s", fpHmacKey)
	}

	eh := e2eutils.NewEOTSServerHandler(t, eotsCfg, eotsHomeDir)
	eh.Start(ctx)

	cfg.RPCListener = fmt.Sprintf("127.0.0.1:%d", testutil.AllocateUniquePort(t))
	eotsCli := NewEOTSManagerGrpcClientWithRetry(t, eotsCfg)

	tm := &TestManager{
		BaseTestManager: &base_test_manager.BaseTestManager{
			AnonController: bc.(*anccc.ClientWrapper),
			CovenantPrivKeys:  covenantPrivKeys,
		},
		EOTSServerHandler: eh,
		EOTSHomeDir:       eotsHomeDir,
		FpConfig:          cfg,
		EOTSClient:        eotsCli,
		ANCConsumerClient: bcc.(*anccc.AnonConsumerController),
		baseDir:           testDir,
		manager:           manager,
		logger:            logger,
		anond:          anond,
	}

	tm.WaitForServicesStart(t)

	return tm
}

func (tm *TestManager) AddFinalityProvider(t *testing.T, ctx context.Context, hmacKey ...string) *service.FinalityProviderInstance {
	r := rand.New(rand.NewSource(time.Now().Unix()))

	eotsKeyName := fmt.Sprintf("eots-key-%s", datagen.GenRandomHexStr(r, 4))
	eotsPkBz, err := tm.EOTSServerHandler.CreateKey(eotsKeyName, "")
	require.NoError(t, err)

	eotsPk, err := anctypes.NewBIP340PubKey(eotsPkBz)
	require.NoError(t, err)

	t.Logf("the EOTS key is created: %s", eotsPk.MarshalHex())

	// Create FP anon key
	fpKeyName := fmt.Sprintf("fp-key-%s", datagen.GenRandomHexStr(r, 4))
	fpHomeDir := filepath.Join(tm.baseDir, fmt.Sprintf("fp-%s", datagen.GenRandomHexStr(r, 4)))
	cfg := e2eutils.DefaultFpConfig(tm.baseDir, fpHomeDir)
	cfg.AnonConfig.Key = fpKeyName
	cfg.AnonConfig.RPCAddr = fmt.Sprintf("http://localhost:%s", tm.anond.GetPort("26657/tcp"))
	cfg.AnonConfig.GRPCAddr = fmt.Sprintf("https://localhost:%s", tm.anond.GetPort("9090/tcp"))

	// Set HMAC key if provided
	if len(hmacKey) > 0 && hmacKey[0] != "" {
		cfg.HMACKey = hmacKey[0]
	}

	fpAncKeyInfo, err := testutil.CreateChainKey(cfg.AnonConfig.KeyDirectory, cfg.AnonConfig.ChainID, cfg.AnonConfig.Key, cfg.AnonConfig.KeyringBackend, passphrase, hdPath, "")
	require.NoError(t, err)

	t.Logf("the Anon key is created: %s", fpAncKeyInfo.AccAddress.String())

	// Add funds for new FP
	_, _, err = tm.manager.AnondTxBankSend(t, fpAncKeyInfo.AccAddress.String(), "1000000uanc", "node0")
	require.NoError(t, err)

	// create new clients
	bc, err := fpcc.NewAnonController(cfg.AnonConfig, tm.logger)
	require.NoError(t, err)
	err = bc.Start()
	require.NoError(t, err)
	bcc, err := anccc.NewAnonConsumerController(cfg.AnonConfig, tm.logger)
	require.NoError(t, err)

	// Create and start finality provider app
	eotsCli, err := client.NewEOTSManagerGRPCClient(tm.EOTSServerHandler.Config().RPCListener, tm.EOTSServerHandler.Config().HMACKey)
	require.NoError(t, err)
	fpdb, err := cfg.DatabaseConfig.GetDBBackend()
	require.NoError(t, err)

	fpMetrics := metrics.NewFpMetrics()
	poller := service.NewChainPoller(tm.logger, cfg.PollerConfig, bcc, fpMetrics)
	pubRandStore, err := fpstore.NewPubRandProofStore(fpdb)
	require.NoError(t, err)
	rndCommitter := service.NewDefaultRandomnessCommitter(
		service.NewRandomnessCommitterConfig(cfg.NumPubRand, int64(cfg.TimestampingDelayBlocks), cfg.ContextSigningHeight),
		service.NewPubRandState(pubRandStore), bcc, eotsCli, tm.logger, fpMetrics)
	heightDeterminer := service.NewStartHeightDeterminer(bcc, cfg.PollerConfig, tm.logger)
	fsCfg := service.NewDefaultFinalitySubmitterConfig(
		cfg.MaxSubmissionRetries,
		cfg.ContextSigningHeight,
		cfg.SubmissionRetryInterval,
	)
	finalitySubmitter := service.NewDefaultFinalitySubmitter(bcc, eotsCli, rndCommitter.GetPubRandProofList, fsCfg, tm.logger, fpMetrics)

	fpApp, err := service.NewFinalityProviderApp(cfg, bc, bcc, eotsCli, poller, rndCommitter, heightDeterminer, finalitySubmitter, fpMetrics, fpdb, tm.logger)
	require.NoError(t, err)
	err = fpApp.Start(ctx)
	require.NoError(t, err)

	// Create and register the finality provider
	// Add retry logic for creating the finality provider
	commission := testutil.ZeroCommissionRate()
	desc := newDescription(testMoniker)

	_, err = fpApp.CreateFinalityProvider(ctx, cfg.AnonConfig.Key, testChainID, eotsPk, desc, commission)
	require.NoError(t, err)

	cfg.RPCListener = fmt.Sprintf("127.0.0.1:%d", testutil.AllocateUniquePort(t))
	cfg.Metrics.Port = testutil.AllocateUniquePort(t)

	err = fpApp.StartFinalityProvider(ctx, eotsPk)
	require.NoError(t, err)

	fpServer := service.NewFinalityProviderServer(cfg, tm.logger, fpApp, fpdb)
	go func() {
		err = fpServer.RunUntilShutdown(ctx)
		require.NoError(t, err)
	}()

	tm.Fps = append(tm.Fps, fpApp)

	fpIns, err := fpApp.GetFinalityProviderInstance()
	require.NoError(t, err)

	return fpIns
}

func (tm *TestManager) WaitForServicesStart(t *testing.T) {
	require.Eventually(t, func() bool {
		_, err := tm.AnonController.QueryBtcLightClientTip()

		return err == nil
	}, eventuallyWaitTimeOut, eventuallyPollTime)

	t.Logf("Anon node is started")
}

func StartManagerWithFinalityProvider(t *testing.T, n int, ctx context.Context, hmacKey ...string) (*TestManager, []*service.FinalityProviderInstance) {
	// If HMAC key is provided, use it for both server and client
	var tm *TestManager
	if len(hmacKey) > 0 && hmacKey[0] != "" {
		// Use the same key for both EOTS server and FP client for simplicity
		tm = StartManager(t, ctx, hmacKey[0], hmacKey[0])
	} else {
		tm = StartManager(t, ctx, "", "")
	}

	var runningFps []*service.FinalityProviderInstance
	for i := 0; i < n; i++ {
		// Pass the HMAC key if provided, otherwise don't use HMAC
		var fpIns *service.FinalityProviderInstance
		if len(hmacKey) > 0 && hmacKey[0] != "" {
			fpIns = tm.AddFinalityProvider(t, ctx, hmacKey[0])
		} else {
			fpIns = tm.AddFinalityProvider(t, ctx)
		}
		runningFps = append(runningFps, fpIns)
	}

	// Check finality providers on Anon side
	require.Eventually(t, func() bool {
		fps, err := tm.AnonController.QueryFinalityProviders()
		if err != nil {
			t.Logf("failed to query finality providers from Anon %s", err.Error())
			return false
		}

		return len(fps) == n
	}, eventuallyWaitTimeOut, eventuallyPollTime)

	t.Logf("the test manager is running with a finality provider")

	return tm, runningFps
}

func (tm *TestManager) Stop(t *testing.T) {
	for _, fpApp := range tm.Fps {
		err := fpApp.Stop()
		if err != nil {
			t.Logf("Warning: Error stopping finality provider: %v", err)
		}
	}
	err := tm.manager.ClearResources()
	if err != nil {
		t.Logf("Warning: Error clearing Docker resources: %v", err)
	}

	err = os.RemoveAll(tm.baseDir)
	if err != nil {
		t.Logf("Warning: Error removing temporary directory: %v", err)
	}
}

func (tm *TestManager) CheckBlockFinalization(t *testing.T, height uint64, num int) {
	// We need to ensure votes are collected at the given height
	require.Eventually(t, func() bool {
		votes, err := tm.AnonController.QueryVotesAtHeight(height)
		if err != nil {
			t.Logf("failed to get the votes at height %v: %s", height, err.Error())
			return false
		}
		return len(votes) >= num // votes could come in faster than we poll
	}, eventuallyWaitTimeOut, eventuallyPollTime)

	// As the votes have been collected, the block should be finalized
	require.Eventually(t, func() bool {
		finalized, err := tm.ANCConsumerClient.QueryIsBlockFinalized(t.Context(), height)
		if err != nil {
			t.Logf("failed to query block at height %v: %s", height, err.Error())
			return false
		}
		return finalized
	}, eventuallyWaitTimeOut, eventuallyPollTime)
}

func (tm *TestManager) WaitForFpVoteCast(t *testing.T, fpIns *service.FinalityProviderInstance) uint64 {
	var lastVotedHeight uint64
	require.Eventually(t, func() bool {
		if fpIns.GetLastVotedHeight() > 0 {
			lastVotedHeight = fpIns.GetLastVotedHeight()
			return true
		}
		return false
	}, eventuallyWaitTimeOut, eventuallyPollTime)

	return lastVotedHeight
}

func (tm *TestManager) WaitForFpVoteCastAtHeight(t *testing.T, fpIns *service.FinalityProviderInstance, height uint64) {
	var lastVotedHeight uint64
	require.Eventually(t, func() bool {
		votedHeight := fpIns.GetLastVotedHeight()
		if votedHeight >= height {
			lastVotedHeight = votedHeight
			return true
		}
		return false
	}, eventuallyWaitTimeOut, eventuallyPollTime)

	t.Logf("the fp voted at height %d", lastVotedHeight)
}

func (tm *TestManager) StopAndRestartFpAfterNBlocks(ctx context.Context, t *testing.T, n int, fpIns *service.FinalityProviderInstance) {
	blockBeforeStop, err := tm.ANCConsumerClient.QueryLatestBlock(ctx)
	require.NotNil(t, blockBeforeStop)
	require.NoError(t, err)
	err = fpIns.Stop()
	require.NoError(t, err)

	require.Eventually(t, func() bool {
		headerAfterStop, err := tm.ANCConsumerClient.QueryLatestBlock(ctx)
		if headerAfterStop == nil || err != nil {
			return false
		}

		return headerAfterStop.GetHeight() >= uint64(n)+blockBeforeStop.GetHeight()
	}, eventuallyWaitTimeOut, eventuallyPollTime)

	t.Log("restarting the finality-provider instance")

	err = fpIns.Start(ctx)
	require.NoError(t, err)

	// Add sleep to allow database to initialize
	time.Sleep(15 * time.Second)
}

func (tm *TestManager) WaitForNBlocks(t *testing.T, n int) uint64 {
	beforeHeight, err := tm.ANCConsumerClient.QueryLatestBlock(t.Context())
	require.NotNil(t, beforeHeight)
	require.NoError(t, err)

	var afterHeight uint64
	require.Eventually(t, func() bool {
		block, err := tm.ANCConsumerClient.QueryLatestBlock(t.Context())
		if block == nil || err != nil {
			return false
		}

		if block.GetHeight() >= uint64(n)+beforeHeight.GetHeight() {
			afterHeight = block.GetHeight()
			return true
		}

		return false
	}, eventuallyWaitTimeOut, eventuallyPollTime)

	return afterHeight
}

func (tm *TestManager) WaitForNFinalizedBlocks(t *testing.T, n uint) *types.BlockInfo {
	var (
		firstFinalizedBlock types.BlockDescription
		err                 error
		lastFinalizedBlock  types.BlockDescription
	)

	require.Eventually(t, func() bool {
		lastFinalizedBlock, err = tm.ANCConsumerClient.QueryLatestFinalizedBlock(t.Context())
		if err != nil {
			t.Logf("failed to get the latest finalized block: %s", err.Error())
			return false
		}
		if lastFinalizedBlock == nil {
			return false
		}
		if firstFinalizedBlock == nil {
			firstFinalizedBlock = lastFinalizedBlock
		}
		return lastFinalizedBlock.GetHeight()-firstFinalizedBlock.GetHeight() >= uint64(n-1)
	}, eventuallyWaitTimeOut, eventuallyPollTime)

	t.Logf("the block is finalized at %v", lastFinalizedBlock.GetHeight())

	return types.NewBlockInfo(lastFinalizedBlock.GetHeight(), lastFinalizedBlock.GetHash(), lastFinalizedBlock.IsFinalized())
}

func newDescription(moniker string) *stakingtypes.Description {
	dec := stakingtypes.NewDescription(moniker, "", "", "", "")
	return &dec
}

func NewEOTSManagerGrpcClientWithRetry(t *testing.T, cfg *eotsconfig.Config) *client.EOTSManagerGRPCClient {
	var err error
	var eotsCli *client.EOTSManagerGRPCClient
	err = retry.Do(func() error {
		eotsCli, err = client.NewEOTSManagerGRPCClient(cfg.RPCListener, cfg.HMACKey)
		return err
	}, retry.Attempts(5))
	require.NoError(t, err)

	return eotsCli
}
