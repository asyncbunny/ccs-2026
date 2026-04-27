package e2etest

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"os/exec"
	"sync"
	"testing"
	"time"

	"github.com/anon-org/anon/v4/testutil/datagen"
	anctypes "github.com/anon-org/anon/v4/types"
	btcctypes "github.com/anon-org/anon/v4/x/btccheckpoint/types"
	btclctypes "github.com/anon-org/anon/v4/x/btclightclient/types"
	bstypes "github.com/anon-org/anon/v4/x/btcstaking/types"
	covcc "github.com/anon-org/covenant-emulator/clientcontroller"
	covcfg "github.com/anon-org/covenant-emulator/config"
	"github.com/anon-org/covenant-emulator/covenant"
	signerCfg "github.com/anon-org/covenant-emulator/covenant-signer/config"
	"github.com/anon-org/covenant-emulator/covenant-signer/keystore/cosmos"
	signerMetrics "github.com/anon-org/covenant-emulator/covenant-signer/observability/metrics"
	signerApp "github.com/anon-org/covenant-emulator/covenant-signer/signerapp"
	signerService "github.com/anon-org/covenant-emulator/covenant-signer/signerservice"
	"github.com/anon-org/covenant-emulator/remotesigner"
	"github.com/anon-org/covenant-emulator/types"
	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/wire"
	sdk "github.com/cosmos/cosmos-sdk/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

var (
	eventuallyWaitTimeOut = 1 * time.Minute
	eventuallyPollTime    = 500 * time.Millisecond
	btcNetworkParams      = &chaincfg.SimNetParams

	chainID    = "chain-test"
	passphrase = "testpass"
	hdPath     = ""
)

type TestManager struct {
	Wg               sync.WaitGroup
	AnonHandler   *AnonNodeHandler
	CovenantEmulator *covenant.Emulator
	CovenanConfig    *covcfg.Config
	CovANCClient     *covcc.AnonController
	StakingParams    *types.StakingParams
	Signer           *remotesigner.RemoteSigner
	baseDir          string
}

type TestDelegationData struct {
	DelegatorPrivKey *btcec.PrivateKey
	DelegatorKey     *btcec.PublicKey
	SlashingTx       *bstypes.BTCSlashingTx
	StakingTx        *wire.MsgTx
	StakingTxInfo    *btcctypes.TransactionInfo
	DelegatorSig     *anctypes.BIP340Signature
	FpPks            []*btcec.PublicKey

	SlashingPkScript []byte
	StakingTime      uint16
	StakingAmount    int64
}

type testFinalityProviderData struct {
	AnonAddress sdk.AccAddress
	BtcPrivKey     *btcec.PrivateKey
	BtcKey         *btcec.PublicKey
	PoP            *bstypes.ProofOfPossessionBTC
}

func StartManager(t *testing.T, hmacKey string) *TestManager {
	testDir, err := baseDir("cee2etest")
	require.NoError(t, err)

	logger := zap.NewNop()
	covenantConfig := defaultCovenantConfig(testDir)
	err = covenantConfig.Validate()
	require.NoError(t, err)

	// 1. prepare covenant key, which will be used as input of Anon node
	signerConfig := signerCfg.DefaultConfig()
	signerConfig.KeyStore.CosmosKeyStore.ChainID = covenantConfig.AnonConfig.ChainID
	signerConfig.KeyStore.CosmosKeyStore.KeyName = covenantConfig.AnonConfig.Key
	signerConfig.KeyStore.CosmosKeyStore.KeyringBackend = covenantConfig.AnonConfig.KeyringBackend
	signerConfig.KeyStore.CosmosKeyStore.KeyDirectory = covenantConfig.AnonConfig.KeyDirectory
	keyRetriever, err := cosmos.NewCosmosKeyringRetriever(signerConfig.KeyStore.CosmosKeyStore)
	require.NoError(t, err)
	keyInfo, err := keyRetriever.Kr.CreateChainKey(
		passphrase,
		hdPath,
	)
	require.NoError(t, err)
	require.NotNil(t, keyInfo)

	app := signerApp.NewSignerApp(
		keyRetriever,
	)

	met := signerMetrics.NewCovenantSignerMetrics()
	parsedConfig, err := signerConfig.Parse()
	require.NoError(t, err)

	remoteSignerPort, url := AllocateUniquePort(t)
	parsedConfig.ServerConfig.Port = remoteSignerPort

	// Configure HMAC keys if provided
	if hmacKey != "" {
		parsedConfig.ServerConfig.HMACKey = hmacKey
		covenantConfig.RemoteSigner.HMACKey = hmacKey
	}

	covenantConfig.MaxRetiresBatchRemovingMsgs = 3
	covenantConfig.RemoteSigner.URL = fmt.Sprintf("http://%s", url)

	server, err := signerService.New(
		context.Background(),
		parsedConfig,
		app,
		met,
	)
	require.NoError(t, err)

	signer := remotesigner.NewRemoteSigner(covenantConfig.RemoteSigner)
	covPubKey := keyInfo.PublicKey

	go func() {
		_ = server.Start()
	}()

	// Give some time to launch server
	time.Sleep(3 * time.Second)

	// unlock the signer before usage
	err = signerService.Unlock(
		context.Background(),
		covenantConfig.RemoteSigner.URL,
		covenantConfig.RemoteSigner.Timeout,
		passphrase,
		covenantConfig.RemoteSigner.HMACKey,
	)
	require.NoError(t, err)

	t.Cleanup(func() {
		_ = server.Stop(context.TODO())
	})

	// 2. prepare Anon node
	bh := NewAnonNodeHandler(t, anctypes.NewBIP340PubKeyFromBTCPK(covPubKey))
	err = bh.Start()
	require.NoError(t, err)

	// 3. prepare covenant emulator
	ancCfg := defaultANCConfigWithKey("test-spending-key", bh.GetNodeDataDir())
	covbc, err := covcc.NewAnonController(ancCfg, &covenantConfig.BTCNetParams, logger, covenantConfig.MaxRetiresBatchRemovingMsgs)
	require.NoError(t, err)

	require.NoError(t, err)

	ce, err := covenant.NewEmulator(covenantConfig, covbc, logger, signer)
	require.NoError(t, err)
	err = ce.Start()
	require.NoError(t, err)

	tm := &TestManager{
		AnonHandler:   bh,
		CovenantEmulator: ce,
		CovenanConfig:    covenantConfig,
		CovANCClient:     covbc,
		baseDir:          testDir,
		Signer:           &signer,
	}

	tm.WaitForServicesStart(t)
	tm.SendToAddr(t, keyInfo.Address.String(), "100000uanc")

	return tm
}

func (tm *TestManager) WaitForServicesStart(t *testing.T) {
	// wait for Anon node starts
	require.Eventually(t, func() bool {
		params, err := tm.CovANCClient.QueryStakingParamsByVersion(0)
		if err != nil {
			return false
		}
		tm.StakingParams = params

		return true
	}, eventuallyWaitTimeOut, eventuallyPollTime)

	t.Logf("Anon node is started")
}

func (tm *TestManager) SendToAddr(t *testing.T, toAddr, amount string) {
	//nolint:noctx
	sendTx := exec.Command(
		"anond",
		"tx",
		"bank",
		"send",
		"node0",
		toAddr,
		amount,
		"--keyring-backend=test",
		"--chain-id=chain-test",
		fmt.Sprintf("--home=%s", tm.AnonHandler.anonNode.nodeHome),
	)
	err := sendTx.Start()
	require.NoError(t, err)
}

func StartManagerWithFinalityProvider(t *testing.T, n int) (*TestManager, []*btcec.PublicKey) {
	tm := StartManager(t, "")
	// fund the finality provider operator account
	// to submit the registration tx
	tm.SendToAddr(t, tm.CovANCClient.GetKeyAddress().String(), "100000uanc")

	var btcPks []*btcec.PublicKey
	for i := 0; i < n; i++ {
		fpData := genTestFinalityProviderData(
			t,
			tm.CovenanConfig.AnonConfig.ChainID,
			tm.CovANCClient.GetKeyAddress(),
		)
		btcPubKey := anctypes.NewBIP340PubKeyFromBTCPK(fpData.BtcKey)
		_, err := tm.CovANCClient.RegisterFinalityProvider(
			btcPubKey,
			&tm.StakingParams.MinComissionRate,
			&stakingtypes.Description{
				Moniker: "tester",
			},
			fpData.PoP,
		)
		require.NoError(t, err)

		btcPks = append(btcPks, fpData.BtcKey)
	}

	// check finality providers on Anon side
	require.Eventually(t, func() bool {
		fps, err := tm.CovANCClient.QueryFinalityProviders()
		if err != nil {
			t.Logf("failed to query finality providers from Anon %s", err.Error())

			return false
		}

		return len(fps) == n
	}, eventuallyWaitTimeOut, eventuallyPollTime)

	t.Logf("the test manager is running with %v finality-provider(s)", n)

	return tm, btcPks
}

func genTestFinalityProviderData(t *testing.T, _ string, anonAddr sdk.AccAddress) *testFinalityProviderData {
	finalityProviderEOTSPrivKey, err := btcec.NewPrivateKey()
	require.NoError(t, err)
	pop, err := datagen.NewPoPBTC(anonAddr, finalityProviderEOTSPrivKey)
	require.NoError(t, err)

	return &testFinalityProviderData{
		AnonAddress: anonAddr,
		BtcPrivKey:     finalityProviderEOTSPrivKey,
		BtcKey:         finalityProviderEOTSPrivKey.PubKey(),
		PoP:            pop,
	}
}

func (tm *TestManager) Stop(t *testing.T) {
	err := tm.CovenantEmulator.Stop()
	require.NoError(t, err)
	err = tm.AnonHandler.Stop()
	require.NoError(t, err)
	err = os.RemoveAll(tm.baseDir)
	require.NoError(t, err)
}

func (tm *TestManager) WaitForNPendingDels(t *testing.T, n int) []*types.Delegation {
	var (
		dels []*types.Delegation
		err  error
	)
	require.Eventually(t, func() bool {
		dels, err = tm.CovANCClient.QueryPendingDelegations(
			tm.CovenanConfig.DelegationLimit,
			nil,
		)
		if err != nil {
			return false
		}

		return len(dels) == n
	}, eventuallyWaitTimeOut, eventuallyPollTime)

	t.Logf("delegations are pending")

	return dels
}

// waitForNDelsWithStatus is a generic method for waiting for delegations with a specific status
func (tm *TestManager) waitForNDelsWithStatus(t *testing.T, n int, queryFunc func(uint64) ([]*types.Delegation, error), statusName string) []*types.Delegation {
	var (
		dels []*types.Delegation
		err  error
	)
	require.Eventually(t, func() bool {
		dels, err = queryFunc(tm.CovenanConfig.DelegationLimit)
		if err != nil {
			return false
		}

		return len(dels) >= n
	}, eventuallyWaitTimeOut, eventuallyPollTime)

	t.Logf("delegations are %s", statusName)

	return dels
}

func (tm *TestManager) WaitForNActiveDels(t *testing.T, n int) []*types.Delegation {
	return tm.waitForNDelsWithStatus(t, n, tm.CovANCClient.QueryActiveDelegations, "active")
}

func (tm *TestManager) WaitForNVerifiedDels(t *testing.T, n int) []*types.Delegation {
	return tm.waitForNDelsWithStatus(t, n, tm.CovANCClient.QueryVerifiedDelegations, "verified")
}

// InsertBTCDelegation inserts a BTC delegation to Anon
// isPreApproval indicates whether the delegation follows
// pre-approval flow, if so, the inclusion proof is nil
func (tm *TestManager) InsertBTCDelegation(
	t *testing.T,
	fpPks []*btcec.PublicKey, stakingTime uint16, stakingAmount int64,
	isPreApproval bool,
) *TestDelegationData {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	params := tm.StakingParams

	// delegator BTC key pairs, staking tx and slashing tx
	delBtcPrivKey, delBtcPubKey, err := datagen.GenRandomBTCKeyPair(r)
	require.NoError(t, err)

	unbondingTime := uint16(tm.StakingParams.UnbondingTimeBlocks)
	testStakingInfo := datagen.GenBTCStakingSlashingInfo(
		r,
		t,
		btcNetworkParams,
		delBtcPrivKey,
		fpPks,
		params.CovenantPks,
		params.CovenantQuorum,
		stakingTime,
		stakingAmount,
		params.SlashingPkScript,
		params.SlashingRate,
		unbondingTime,
	)

	// proof-of-possession
	pop, err := datagen.NewPoPBTC(tm.CovANCClient.GetKeyAddress(), delBtcPrivKey)
	require.NoError(t, err)

	// create and insert BTC headers which include the staking tx to get staking tx info
	currentBtcTipResp, err := tm.CovANCClient.QueryBtcLightClientTip()
	require.NoError(t, err)
	tipHeader, err := anctypes.NewBTCHeaderBytesFromHex(currentBtcTipResp.HeaderHex)
	require.NoError(t, err)
	blockWithStakingTx := datagen.CreateBlockWithTransaction(r, tipHeader.ToBlockHeader(), testStakingInfo.StakingTx)
	accumulatedWork := btclctypes.CalcWork(&blockWithStakingTx.HeaderBytes)
	accumulatedWork = btclctypes.CumulativeWork(accumulatedWork, currentBtcTipResp.Work)
	parentBlockHeaderInfo := &btclctypes.BTCHeaderInfo{
		Header: &blockWithStakingTx.HeaderBytes,
		Hash:   blockWithStakingTx.HeaderBytes.Hash(),
		Height: currentBtcTipResp.Height + 1,
		Work:   &accumulatedWork,
	}
	headers := make([]anctypes.BTCHeaderBytes, 0)
	headers = append(headers, blockWithStakingTx.HeaderBytes)
	for i := 0; i < int(params.ComfirmationTimeBlocks); i++ {
		headerInfo := datagen.GenRandomValidBTCHeaderInfoWithParent(r, *parentBlockHeaderInfo)
		headers = append(headers, *headerInfo.Header)
		parentBlockHeaderInfo = headerInfo
	}
	_, err = tm.CovANCClient.InsertBtcBlockHeaders(headers)
	require.NoError(t, err)
	btcHeader := blockWithStakingTx.HeaderBytes
	serializedStakingTx, err := anctypes.SerializeBTCTx(testStakingInfo.StakingTx)
	require.NoError(t, err)
	txInfo := btcctypes.NewTransactionInfo(&btcctypes.TransactionKey{Index: 1, Hash: btcHeader.Hash()}, serializedStakingTx, blockWithStakingTx.SpvProof.MerkleNodes)

	slashingSpendInfo, err := testStakingInfo.StakingInfo.SlashingPathSpendInfo()
	require.NoError(t, err)

	// delegator sig
	delegatorSig, err := testStakingInfo.SlashingTx.Sign(
		testStakingInfo.StakingTx,
		datagen.StakingOutIdx,
		slashingSpendInfo.GetPkScriptPath(),
		delBtcPrivKey,
	)
	require.NoError(t, err)

	unbondingValue := stakingAmount - 1000
	stakingTxHash := testStakingInfo.StakingTx.TxHash()

	testUnbondingInfo := datagen.GenBTCUnbondingSlashingInfo(
		r,
		t,
		btcNetworkParams,
		delBtcPrivKey,
		fpPks,
		params.CovenantPks,
		params.CovenantQuorum,
		wire.NewOutPoint(&stakingTxHash, datagen.StakingOutIdx),
		unbondingTime,
		unbondingValue,
		params.SlashingPkScript,
		params.SlashingRate,
		unbondingTime,
	)

	unbondingTxMsg := testUnbondingInfo.UnbondingTx

	unbondingSlashingPathInfo, err := testUnbondingInfo.UnbondingInfo.SlashingPathSpendInfo()
	require.NoError(t, err)

	unbondingSig, err := testUnbondingInfo.SlashingTx.Sign(
		unbondingTxMsg,
		datagen.StakingOutIdx,
		unbondingSlashingPathInfo.GetPkScriptPath(),
		delBtcPrivKey,
	)
	require.NoError(t, err)

	serializedUnbondingTx, err := anctypes.SerializeBTCTx(testUnbondingInfo.UnbondingTx)
	require.NoError(t, err)

	// submit the BTC delegation to Anon
	_, err = tm.CovANCClient.CreateBTCDelegation(
		anctypes.NewBIP340PubKeyFromBTCPK(delBtcPubKey),
		fpPks,
		pop,
		uint32(stakingTime),
		stakingAmount,
		txInfo,
		testStakingInfo.SlashingTx,
		delegatorSig,
		serializedUnbondingTx,
		uint32(unbondingTime),
		unbondingValue,
		testUnbondingInfo.SlashingTx,
		unbondingSig,
		isPreApproval)
	require.NoError(t, err)

	t.Log("successfully submitted a BTC delegation")

	return &TestDelegationData{
		DelegatorPrivKey: delBtcPrivKey,
		DelegatorKey:     delBtcPubKey,
		FpPks:            fpPks,
		StakingTx:        testStakingInfo.StakingTx,
		SlashingTx:       testStakingInfo.SlashingTx,
		StakingTxInfo:    txInfo,
		DelegatorSig:     delegatorSig,
		SlashingPkScript: params.SlashingPkScript,
		StakingTime:      stakingTime,
		StakingAmount:    stakingAmount,
	}
}

// InsertStakeExpansionDelegation inserts a BTC stake expansion delegation to Anon
func (tm *TestManager) InsertStakeExpansionDelegation(
	t *testing.T,
	fpPks []*btcec.PublicKey,
	stakingTime uint16,
	stakingAmount int64,
	prevStakingTx *wire.MsgTx,
	_ bool,
) *TestDelegationData {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	params := tm.StakingParams

	// delegator BTC key pairs, staking tx and slashing tx
	delBtcPrivKey, delBtcPubKey, err := datagen.GenRandomBTCKeyPair(r)
	require.NoError(t, err)

	unbondingTime := uint16(tm.StakingParams.UnbondingTimeBlocks)

	// Convert previousStakingTxHash string to OutPoint
	previousStakingTxHash := prevStakingTx.TxHash().String()
	prevHash, err := chainhash.NewHashFromStr(previousStakingTxHash)
	require.NoError(t, err)
	prevStakingOutPoint := wire.NewOutPoint(prevHash, datagen.StakingOutIdx)

	fundingTx := datagen.GenRandomTxWithOutputValue(r, stakingAmount)

	// Convert fundingTxHash to OutPoint
	fundingTxHash := fundingTx.TxHash()
	fundingOutPoint := wire.NewOutPoint(&fundingTxHash, 0)
	outPoints := []*wire.OutPoint{prevStakingOutPoint, fundingOutPoint}

	// Generate staking slashing info using the previous staking outpoint
	// For stake expansion, we create a transaction with multiple inputs
	testStakingInfo := datagen.GenBTCStakingSlashingInfoWithInputs(
		r,
		t,
		btcNetworkParams,
		outPoints,
		delBtcPrivKey,
		fpPks,
		params.CovenantPks,
		params.CovenantQuorum,
		stakingTime,
		stakingAmount,
		params.SlashingPkScript,
		params.SlashingRate,
		unbondingTime,
	)

	// proof-of-possession
	pop, err := datagen.NewPoPBTC(tm.CovANCClient.GetKeyAddress(), delBtcPrivKey)
	require.NoError(t, err)

	// create and insert BTC headers which include the staking tx to get staking tx info
	currentBtcTipResp, err := tm.CovANCClient.QueryBtcLightClientTip()
	require.NoError(t, err)
	tipHeader, err := anctypes.NewBTCHeaderBytesFromHex(currentBtcTipResp.HeaderHex)
	require.NoError(t, err)
	blockWithStakingTx := datagen.CreateBlockWithTransaction(r, tipHeader.ToBlockHeader(), testStakingInfo.StakingTx)
	accumulatedWork := btclctypes.CalcWork(&blockWithStakingTx.HeaderBytes)
	accumulatedWork = btclctypes.CumulativeWork(accumulatedWork, currentBtcTipResp.Work)
	parentBlockHeaderInfo := &btclctypes.BTCHeaderInfo{
		Header: &blockWithStakingTx.HeaderBytes,
		Hash:   blockWithStakingTx.HeaderBytes.Hash(),
		Height: currentBtcTipResp.Height + 1,
		Work:   &accumulatedWork,
	}
	headers := make([]anctypes.BTCHeaderBytes, 0)
	headers = append(headers, blockWithStakingTx.HeaderBytes)
	for i := 0; i < int(params.ComfirmationTimeBlocks); i++ {
		headerInfo := datagen.GenRandomValidBTCHeaderInfoWithParent(r, *parentBlockHeaderInfo)
		headers = append(headers, *headerInfo.Header)
		parentBlockHeaderInfo = headerInfo
	}
	_, err = tm.CovANCClient.InsertBtcBlockHeaders(headers)
	require.NoError(t, err)
	btcHeader := blockWithStakingTx.HeaderBytes
	serializedStakingTx, err := anctypes.SerializeBTCTx(testStakingInfo.StakingTx)
	require.NoError(t, err)
	txInfo := btcctypes.NewTransactionInfo(&btcctypes.TransactionKey{Index: 1, Hash: btcHeader.Hash()}, serializedStakingTx, blockWithStakingTx.SpvProof.MerkleNodes)

	slashingSpendInfo, err := testStakingInfo.StakingInfo.SlashingPathSpendInfo()
	require.NoError(t, err)

	// delegator sig
	delegatorSig, err := testStakingInfo.SlashingTx.Sign(
		testStakingInfo.StakingTx,
		datagen.StakingOutIdx,
		slashingSpendInfo.GetPkScriptPath(),
		delBtcPrivKey,
	)
	require.NoError(t, err)

	unbondingValue := stakingAmount - 1000
	stakingTxHash := testStakingInfo.StakingTx.TxHash()

	testUnbondingInfo := datagen.GenBTCUnbondingSlashingInfo(
		r,
		t,
		btcNetworkParams,
		delBtcPrivKey,
		fpPks,
		params.CovenantPks,
		params.CovenantQuorum,
		wire.NewOutPoint(&stakingTxHash, datagen.StakingOutIdx),
		unbondingTime,
		unbondingValue,
		params.SlashingPkScript,
		params.SlashingRate,
		unbondingTime,
	)

	unbondingTxMsg := testUnbondingInfo.UnbondingTx

	unbondingSlashingPathInfo, err := testUnbondingInfo.UnbondingInfo.SlashingPathSpendInfo()
	require.NoError(t, err)

	unbondingSig, err := testUnbondingInfo.SlashingTx.Sign(
		unbondingTxMsg,
		datagen.StakingOutIdx,
		unbondingSlashingPathInfo.GetPkScriptPath(),
		delBtcPrivKey,
	)
	require.NoError(t, err)

	serializedUnbondingTx, err := anctypes.SerializeBTCTx(testUnbondingInfo.UnbondingTx)
	require.NoError(t, err)

	// Serialize the funding transaction for the stake expansion
	serializedFundingTx, err := anctypes.SerializeBTCTx(fundingTx)
	require.NoError(t, err)

	// submit the BTC stake expansion delegation to Anon
	_, err = tm.CovANCClient.CreateStakeExpansionDelegation(
		anctypes.NewBIP340PubKeyFromBTCPK(delBtcPubKey),
		fpPks,
		pop,
		uint32(stakingTime),
		stakingAmount,
		txInfo,
		testStakingInfo.SlashingTx,
		delegatorSig,
		serializedUnbondingTx,
		uint32(unbondingTime),
		unbondingValue,
		testUnbondingInfo.SlashingTx,
		unbondingSig,
		previousStakingTxHash,
		serializedFundingTx,
	)
	require.NoError(t, err)

	t.Log("successfully submitted a BTC stake expansion delegation")

	return &TestDelegationData{
		DelegatorPrivKey: delBtcPrivKey,
		DelegatorKey:     delBtcPubKey,
		FpPks:            fpPks,
		StakingTx:        testStakingInfo.StakingTx,
		SlashingTx:       testStakingInfo.SlashingTx,
		StakingTxInfo:    txInfo,
		DelegatorSig:     delegatorSig,
		SlashingPkScript: params.SlashingPkScript,
		StakingTime:      stakingTime,
		StakingAmount:    stakingAmount,
	}
}

func defaultANCConfigWithKey(key, keydir string) *covcfg.ANCConfig {
	ancCfg := covcfg.DefaultANCConfig()
	ancCfg.Key = key
	ancCfg.KeyDirectory = keydir
	ancCfg.GasAdjustment = 20

	return &ancCfg
}

func defaultCovenantConfig(homeDir string) *covcfg.Config {
	cfg := covcfg.DefaultConfigWithHomePath(homeDir)
	cfg.AnonConfig.KeyDirectory = homeDir

	return &cfg
}
