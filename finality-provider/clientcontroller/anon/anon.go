package anon

import (
	"context"
	"fmt"
	"strings"

	"github.com/anon-org/anon/v4/client/anonclient"

	"github.com/anon-org/finality-provider/finality-provider/proto"

	sdkErr "cosmossdk.io/errors"
	ancclient "github.com/anon-org/anon/v4/client/client"
	anctypes "github.com/anon-org/anon/v4/types"
	btcctypes "github.com/anon-org/anon/v4/x/btccheckpoint/types"
	btclctypes "github.com/anon-org/anon/v4/x/btclightclient/types"
	btcstakingtypes "github.com/anon-org/anon/v4/x/btcstaking/types"
	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcec/v2/schnorr"
	"github.com/btcsuite/btcd/btcutil"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkquery "github.com/cosmos/cosmos-sdk/types/query"
	sttypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	"go.uber.org/zap"
	protobuf "google.golang.org/protobuf/proto"

	"github.com/anon-org/finality-provider/clientcontroller/api"
	fpcfg "github.com/anon-org/finality-provider/finality-provider/config"
	"github.com/anon-org/finality-provider/types"
)

var _ api.AnonController = &ClientWrapper{}

var emptyErrs = []*sdkErr.Error{}

// ClientWrapper - wrapper around the `ancclient.Client` that implements
// the api.AnonController interface. It serves as the primary interface for finality
// providers to interact with the Anon Genesis.
//
// The wrapper handles:
// - Finality provider registration and management
// - BTC delegation operations and covenant signatures
// - Blockchain queries for staking parameters and epoch information
// - BTC light client operations and header management
// - Consumer chain registration and queries
type ClientWrapper struct {
	ancClient *ancclient.Client
	cfg       *fpcfg.ANCConfig
	logger    *zap.Logger
}

func NewAnonController(
	ancClient *ancclient.Client,
	cfg *fpcfg.ANCConfig,
	logger *zap.Logger,
) (*ClientWrapper, error) {
	return &ClientWrapper{
		ancClient,
		cfg,
		logger,
	}, nil
}

func (bc *ClientWrapper) Start() error {
	// makes sure that the key in config really exists and is a valid bech32 addr
	// to allow using mustGetTxSigner
	if _, err := bc.ancClient.GetAddr(); err != nil {
		return fmt.Errorf("failed to get addr: %w", err)
	}

	enabled, err := bc.NodeTxIndexEnabled()
	if err != nil {
		return err
	}

	if !enabled {
		return fmt.Errorf("tx indexing in the anon node must be enabled")
	}

	return nil
}

func (bc *ClientWrapper) MustGetTxSigner() string {
	signer := bc.GetKeyAddress()
	prefix := bc.cfg.AccountPrefix

	return sdk.MustBech32ifyAddressBytes(prefix, signer)
}

func (bc *ClientWrapper) GetKeyAddress() sdk.AccAddress {
	// get key address, retrieves address based on the key name which is configured in
	// cfg *stakercfg.ANCConfig. If this fails, it means we have a misconfiguration problem
	// and we should panic.
	// This is checked at the start of ClientWrapper, so if it fails something is really wrong

	keyRec, err := bc.ancClient.GetKeyring().Key(bc.cfg.Key)
	if err != nil {
		panic(fmt.Sprintf("Failed to get key address: %s", err))
	}

	addr, err := keyRec.GetAddress()
	if err != nil {
		panic(fmt.Sprintf("Failed to get key address: %s", err))
	}

	return addr
}

func (bc *ClientWrapper) reliablySendMsg(ctx context.Context, msg sdk.Msg, expectedErrs []*sdkErr.Error, unrecoverableErrs []*sdkErr.Error) (*anonclient.RelayerTxResponse, error) {
	return bc.reliablySendMsgs(ctx, []sdk.Msg{msg}, expectedErrs, unrecoverableErrs)
}

func (bc *ClientWrapper) reliablySendMsgs(ctx context.Context, msgs []sdk.Msg, expectedErrs []*sdkErr.Error, unrecoverableErrs []*sdkErr.Error) (*anonclient.RelayerTxResponse, error) {
	resp, err := bc.ancClient.ReliablySendMsgs(
		ctx,
		msgs,
		expectedErrs,
		unrecoverableErrs,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to reliably send messages: %w", err)
	}

	return resp, nil
}

// RegisterFinalityProvider registers a finality provider via a MsgCreateFinalityProvider to Anon
// it returns tx hash and error
// If chainID is empty, then it means the FP is a Anon FP
func (bc *ClientWrapper) RegisterFinalityProvider(
	ctx context.Context, req *api.RegisterFinalityProviderRequest,
) (*types.TxResponse, error) {
	var ancPop btcstakingtypes.ProofOfPossessionBTC
	if err := ancPop.Unmarshal(req.Pop); err != nil {
		return nil, fmt.Errorf("invalid proof-of-possession: %w", err)
	}

	var sdkDescription sttypes.Description
	if err := sdkDescription.Unmarshal(req.Description); err != nil {
		return nil, fmt.Errorf("invalid description: %w", err)
	}

	fpAddr := bc.MustGetTxSigner()
	msg := &btcstakingtypes.MsgCreateFinalityProvider{
		Addr:        fpAddr,
		BtcPk:       anctypes.NewBIP340PubKeyFromBTCPK(req.FpPk),
		Pop:         &ancPop,
		Commission:  req.Commission,
		Description: &sdkDescription,
	}

	res, err := bc.reliablySendMsg(ctx, msg, emptyErrs, emptyErrs)
	if err != nil {
		return nil, err
	}

	return &types.TxResponse{TxHash: res.TxHash}, nil
}

func (bc *ClientWrapper) EditFinalityProvider(
	ctx context.Context,
	req *api.EditFinalityProviderRequest,
) (*btcstakingtypes.MsgEditFinalityProvider, error) {
	var reqDesc proto.Description
	if err := protobuf.Unmarshal(req.Description, &reqDesc); err != nil {
		return nil, fmt.Errorf("failed to unmarshal description: %w", err)
	}
	fpPubKey := anctypes.NewBIP340PubKeyFromBTCPK(req.FpPk)

	fpRes, err := bc.QueryFinalityProvider(ctx, req.FpPk)
	if err != nil {
		return nil, err
	}

	if !strings.EqualFold(fpRes.FinalityProvider.Addr, bc.MustGetTxSigner()) {
		return nil, fmt.Errorf("the signer does not correspond to the finality provider's "+
			"Anon address, expected %s got %s", bc.MustGetTxSigner(), fpRes.FinalityProvider.Addr)
	}

	getValueOrDefault := func(reqValue, defaultValue string) string {
		if reqValue != "" {
			return reqValue
		}

		return defaultValue
	}

	resDesc := fpRes.FinalityProvider.Description

	desc := &sttypes.Description{
		Moniker:         getValueOrDefault(reqDesc.Moniker, resDesc.Moniker),
		Identity:        getValueOrDefault(reqDesc.Identity, resDesc.Identity),
		Website:         getValueOrDefault(reqDesc.Website, resDesc.Website),
		SecurityContact: getValueOrDefault(reqDesc.SecurityContact, resDesc.SecurityContact),
		Details:         getValueOrDefault(reqDesc.Details, resDesc.Details),
	}

	msg := &btcstakingtypes.MsgEditFinalityProvider{
		Addr:        bc.MustGetTxSigner(),
		BtcPk:       fpPubKey.MustMarshal(),
		Description: desc,
	}

	if req.Commission != nil {
		msg.Commission = req.Commission
	}

	_, err = bc.reliablySendMsg(ctx, msg, emptyErrs, emptyErrs)
	if err != nil {
		return nil, fmt.Errorf("failed to query the finality provider %s: %w", req.FpPk.SerializeCompressed(), err)
	}

	return msg, nil
}

func (bc *ClientWrapper) QueryFinalityProvider(_ context.Context, fpPk *btcec.PublicKey) (*btcstakingtypes.QueryFinalityProviderResponse, error) {
	fpPubKey := anctypes.NewBIP340PubKeyFromBTCPK(fpPk)
	res, err := bc.ancClient.FinalityProvider(fpPubKey.MarshalHex())
	if err != nil {
		return nil, fmt.Errorf("failed to query the finality provider %s: %w", fpPubKey.MarshalHex(), err)
	}

	return res, nil
}

func (bc *ClientWrapper) NodeTxIndexEnabled() (bool, error) {
	res, err := bc.ancClient.GetStatus()
	if err != nil {
		return false, fmt.Errorf("failed to query node status: %w", err)
	}

	return res.TxIndexEnabled(), nil
}

func (bc *ClientWrapper) Close() error {
	if !bc.ancClient.IsRunning() {
		return nil
	}

	if err := bc.ancClient.Stop(); err != nil {
		return fmt.Errorf("failed to stop Anon client: %w", err)
	}

	return nil
}

/*
	Implementations for e2e tests only
*/

func (bc *ClientWrapper) CreateBTCDelegation(
	delBtcPk *anctypes.BIP340PubKey,
	fpPks []*btcec.PublicKey,
	pop *btcstakingtypes.ProofOfPossessionBTC,
	stakingTime uint32,
	stakingValue int64,
	stakingTxInfo *btcctypes.TransactionInfo,
	slashingTx *btcstakingtypes.BTCSlashingTx,
	delSlashingSig *anctypes.BIP340Signature,
	unbondingTx []byte,
	unbondingTime uint32,
	unbondingValue int64,
	unbondingSlashingTx *btcstakingtypes.BTCSlashingTx,
	delUnbondingSlashingSig *anctypes.BIP340Signature,
) (*types.TxResponse, error) {
	fpBtcPks := make([]anctypes.BIP340PubKey, 0, len(fpPks))
	for _, v := range fpPks {
		fpBtcPks = append(fpBtcPks, *anctypes.NewBIP340PubKeyFromBTCPK(v))
	}
	msg := &btcstakingtypes.MsgCreateBTCDelegation{
		StakerAddr:                    bc.MustGetTxSigner(),
		Pop:                           pop,
		BtcPk:                         delBtcPk,
		FpBtcPkList:                   fpBtcPks,
		StakingTime:                   stakingTime,
		StakingValue:                  stakingValue,
		StakingTx:                     stakingTxInfo.Transaction,
		StakingTxInclusionProof:       btcstakingtypes.NewInclusionProof(stakingTxInfo.Key, stakingTxInfo.Proof),
		SlashingTx:                    slashingTx,
		DelegatorSlashingSig:          delSlashingSig,
		UnbondingTx:                   unbondingTx,
		UnbondingTime:                 unbondingTime,
		UnbondingValue:                unbondingValue,
		UnbondingSlashingTx:           unbondingSlashingTx,
		DelegatorUnbondingSlashingSig: delUnbondingSlashingSig,
	}

	res, err := bc.reliablySendMsg(context.Background(), msg, emptyErrs, emptyErrs)
	if err != nil {
		return nil, err
	}

	return &types.TxResponse{TxHash: res.TxHash}, nil
}

func (bc *ClientWrapper) InsertBtcBlockHeaders(headers []anctypes.BTCHeaderBytes) (*anonclient.RelayerTxResponse, error) {
	msg := &btclctypes.MsgInsertHeaders{
		Signer:  bc.MustGetTxSigner(),
		Headers: headers,
	}

	res, err := bc.reliablySendMsg(context.Background(), msg, emptyErrs, emptyErrs)
	if err != nil {
		return nil, err
	}

	return res, nil
}

// QueryFinalityProviders - TODO: only used in test. this should not be put here. it causes confusion that this is a method
// that will be used when FP runs. in that's the case, it implies it should work all all consumer
// types. but `ancClient.QueryClient.FinalityProviders` doesn't work for consumer chains
func (bc *ClientWrapper) QueryFinalityProviders() ([]*btcstakingtypes.FinalityProviderResponse, error) {
	var fps []*btcstakingtypes.FinalityProviderResponse
	pagination := &sdkquery.PageRequest{
		Limit: 100,
	}

	for {
		// NOTE: empty CSN ID means querying all Anon finality providers
		res, err := bc.ancClient.FinalityProviders(pagination)
		if err != nil {
			return nil, fmt.Errorf("failed to query finality providers: %w", err)
		}
		fps = append(fps, res.FinalityProviders...)
		if res.Pagination == nil || res.Pagination.NextKey == nil {
			break
		}

		pagination.Key = res.Pagination.NextKey
	}

	return fps, nil
}

func (bc *ClientWrapper) QueryConsumerFinalityProviders() ([]*btcstakingtypes.FinalityProviderResponse, error) {
	var fps []*btcstakingtypes.FinalityProviderResponse
	pagination := &sdkquery.PageRequest{
		Limit: 100,
	}

	for {
		res, err := bc.ancClient.FinalityProviders(pagination)
		if err != nil {
			return nil, fmt.Errorf("failed to query finality providers: %w", err)
		}
		fps = append(fps, res.FinalityProviders...)
		if res.Pagination == nil || res.Pagination.NextKey == nil {
			break
		}

		pagination.Key = res.Pagination.NextKey
	}

	return fps, nil
}

func (bc *ClientWrapper) QueryBtcLightClientTip() (*btclctypes.BTCHeaderInfoResponse, error) {
	res, err := bc.ancClient.BTCHeaderChainTip()
	if err != nil {
		return nil, fmt.Errorf("failed to query BTC tip: %w", err)
	}

	return res.Header, nil
}

func (bc *ClientWrapper) QueryCurrentEpoch() (uint64, error) {
	res, err := bc.ancClient.CurrentEpoch()
	if err != nil {
		return 0, fmt.Errorf("failed to query BTC tip: %w", err)
	}

	return res.CurrentEpoch, nil
}

func (bc *ClientWrapper) QueryVotesAtHeight(height uint64) ([]anctypes.BIP340PubKey, error) {
	res, err := bc.ancClient.VotesAtHeight(height)
	if err != nil {
		return nil, fmt.Errorf("failed to query BTC delegations: %w", err)
	}

	return res.BtcPks, nil
}

func (bc *ClientWrapper) QueryPendingDelegations(limit uint64) ([]*btcstakingtypes.BTCDelegationResponse, error) {
	return bc.queryDelegationsWithStatus(btcstakingtypes.BTCDelegationStatus_PENDING, limit)
}

func (bc *ClientWrapper) QueryActiveDelegations(limit uint64) ([]*btcstakingtypes.BTCDelegationResponse, error) {
	return bc.queryDelegationsWithStatus(btcstakingtypes.BTCDelegationStatus_ACTIVE, limit)
}

// queryDelegationsWithStatus queries BTC delegations
// with the given status (either pending or unbonding)
// it is only used when the program is running in Covenant mode
func (bc *ClientWrapper) queryDelegationsWithStatus(status btcstakingtypes.BTCDelegationStatus, limit uint64) ([]*btcstakingtypes.BTCDelegationResponse, error) {
	pagination := &sdkquery.PageRequest{
		Limit: limit,
	}

	res, err := bc.ancClient.BTCDelegations(status, pagination)
	if err != nil {
		return nil, fmt.Errorf("failed to query BTC delegations: %w", err)
	}

	return res.BtcDelegations, nil
}

func (bc *ClientWrapper) QueryStakingParams() (*types.StakingParams, error) {
	// query btc checkpoint params
	ckptParamRes, err := bc.ancClient.BTCCheckpointParams()
	if err != nil {
		return nil, fmt.Errorf("failed to query params of the btccheckpoint module: %w", err)
	}

	// query btc staking params
	stakingParamRes, err := bc.ancClient.BTCStakingParams()
	if err != nil {
		return nil, fmt.Errorf("failed to query staking params: %w", err)
	}

	covenantPks := make([]*btcec.PublicKey, 0, len(stakingParamRes.Params.CovenantPks))
	for _, pk := range stakingParamRes.Params.CovenantPks {
		covPk, err := pk.ToBTCPK()
		if err != nil {
			return nil, fmt.Errorf("invalid covenant public key")
		}
		covenantPks = append(covenantPks, covPk)
	}

	return &types.StakingParams{
		ComfirmationTimeBlocks:    ckptParamRes.Params.BtcConfirmationDepth,
		FinalizationTimeoutBlocks: ckptParamRes.Params.CheckpointFinalizationTimeout,
		MinSlashingTxFeeSat:       btcutil.Amount(stakingParamRes.Params.MinSlashingTxFeeSat),
		CovenantPks:               covenantPks,
		SlashingPkScript:          stakingParamRes.Params.SlashingPkScript,
		CovenantQuorum:            stakingParamRes.Params.CovenantQuorum,
		SlashingRate:              stakingParamRes.Params.SlashingRate,
		UnbondingTime:             stakingParamRes.Params.UnbondingTimeBlocks,
	}, nil
}

func (bc *ClientWrapper) SubmitCovenantSigs(
	covPk *btcec.PublicKey,
	stakingTxHash string,
	slashingSigs [][]byte,
	unbondingSig *schnorr.Signature,
	unbondingSlashingSigs [][]byte,
) (*types.TxResponse, error) {
	bip340UnbondingSig := anctypes.NewBIP340SignatureFromBTCSig(unbondingSig)

	msg := &btcstakingtypes.MsgAddCovenantSigs{
		Signer:                  bc.MustGetTxSigner(),
		Pk:                      anctypes.NewBIP340PubKeyFromBTCPK(covPk),
		StakingTxHash:           stakingTxHash,
		SlashingTxSigs:          slashingSigs,
		UnbondingTxSig:          bip340UnbondingSig,
		SlashingUnbondingTxSigs: unbondingSlashingSigs,
	}

	res, err := bc.reliablySendMsg(context.Background(), msg, emptyErrs, emptyErrs)
	if err != nil {
		return nil, err
	}

	return &types.TxResponse{TxHash: res.TxHash}, nil
}

func (bc *ClientWrapper) InsertSpvProofs(submitter string, proofs []*btcctypes.BTCSpvProof) (*anonclient.RelayerTxResponse, error) {
	msg := &btcctypes.MsgInsertBTCSpvProof{
		Submitter: submitter,
		Proofs:    proofs,
	}

	res, err := bc.reliablySendMsg(context.Background(), msg, emptyErrs, emptyErrs)
	if err != nil {
		return nil, err
	}

	return res, nil
}

func (bc *ClientWrapper) GetANCClient() *ancclient.Client {
	return bc.ancClient
}
