package atomicslasher

import (
	"context"
	"fmt"
	"time"

	"cosmossdk.io/errors"
	"github.com/avast/retry-go/v4"
	anc "github.com/anon-org/anon/v4/types"
	bstypes "github.com/anon-org/anon/v4/x/btcstaking/types"
	"github.com/anon-org/vigilante/config"
	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/cosmos/cosmos-sdk/types/query"
	"go.uber.org/zap"
)

type AnonAdapter struct {
	logger            *zap.Logger
	cfg               *config.BTCStakingTrackerConfig
	retrySleepTime    time.Duration
	maxRetrySleepTime time.Duration
	maxRetryTimes     uint
	ancClient         AnonClient
}

func NewAnonAdapter(
	logger *zap.Logger,
	cfg *config.BTCStakingTrackerConfig,
	retrySleepTime time.Duration,
	maxRetrySleepTime time.Duration,
	maxRetryTimes uint,
	ancClient AnonClient,
) *AnonAdapter {
	return &AnonAdapter{
		logger:            logger,
		cfg:               cfg,
		retrySleepTime:    retrySleepTime,
		maxRetrySleepTime: maxRetrySleepTime,
		maxRetryTimes:     maxRetryTimes,
		ancClient:         ancClient,
	}
}

func (ba *AnonAdapter) BTCStakingParams(ctx context.Context, version uint32) (*bstypes.Params, error) {
	var bsParams *bstypes.Params
	err := retry.Do(
		func() error {
			resp, err := ba.ancClient.BTCStakingParamsByVersion(version)
			if err != nil {
				return err
			}
			bsParams = &resp.Params

			return nil
		},
		retry.Context(ctx),
		retry.Delay(ba.retrySleepTime),
		retry.MaxDelay(ba.maxRetrySleepTime),
		retry.Attempts(ba.maxRetryTimes),
	)

	return bsParams, err
}

func (ba *AnonAdapter) BTCDelegation(ctx context.Context, stakingTxHashHex string) (*bstypes.QueryBTCDelegationResponse, error) {
	var (
		resp *bstypes.QueryBTCDelegationResponse
		err  error
	)
	err = retry.Do(
		func() error {
			resp, err = ba.ancClient.BTCDelegation(stakingTxHashHex)
			if err != nil {
				return err
			}

			return nil
		},
		retry.Context(ctx),
		retry.Delay(ba.retrySleepTime),
		retry.MaxDelay(ba.maxRetrySleepTime),
		retry.Attempts(ba.maxRetryTimes),
	)

	return resp, err
}

// TODO: avoid getting expired BTC delegations
func (ba *AnonAdapter) HandleAllBTCDelegations(handleFunc func(btcDel *bstypes.BTCDelegationResponse) error) error {
	pagination := query.PageRequest{Limit: ba.cfg.NewDelegationsBatchSize}

	for {
		resp, err := ba.ancClient.BTCDelegations(bstypes.BTCDelegationStatus_ANY, &pagination)
		if err != nil {
			return fmt.Errorf("failed to get BTC delegations: %w", err)
		}
		for _, btcDel := range resp.BtcDelegations {
			if err := handleFunc(btcDel); err != nil {
				// we should continue getting and handling evidences in subsequent pages
				// rather than return here
				ba.logger.Error("failed to handle BTC delegations", zap.Error(err))
			}
		}
		if resp.Pagination == nil || resp.Pagination.NextKey == nil {
			break
		}
		pagination.Key = resp.Pagination.NextKey
	}

	return nil
}

func (ba *AnonAdapter) IsFPSlashed(
	_ context.Context,
	fpBTCPK *anc.BIP340PubKey,
) (bool, error) {
	fpResp, err := ba.ancClient.FinalityProvider(fpBTCPK.MarshalHex())
	if err != nil {
		return false, err
	}

	return fpResp.FinalityProvider.SlashedAnonHeight > 0, nil
}

func (ba *AnonAdapter) ReportSelectiveSlashing(
	ctx context.Context,
	fpBTCSK *btcec.PrivateKey,
) error {
	msg := &bstypes.MsgSelectiveSlashingEvidence{
		Signer:           ba.ancClient.MustGetAddr(),
		RecoveredFpBtcSk: fpBTCSK.Serialize(),
	}

	// TODO: what are unrecoverable/expected errors?
	_, err := ba.ancClient.ReliablySendMsg(ctx, msg, []*errors.Error{}, []*errors.Error{})

	return err
}
