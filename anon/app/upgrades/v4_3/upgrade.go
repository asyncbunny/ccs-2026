package v4_3

import (
	"context"
	"errors"
	"fmt"

	"cosmossdk.io/collections"
	corestoretypes "cosmossdk.io/core/store"
	"cosmossdk.io/math"
	store "cosmossdk.io/store/types"
	upgradetypes "cosmossdk.io/x/upgrade/types"
	"github.com/cosmos/cosmos-sdk/codec"
	"github.com/cosmos/cosmos-sdk/runtime"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/module"
	"github.com/cosmos/cosmos-sdk/types/query"
	stkkeeper "github.com/cosmos/cosmos-sdk/x/staking/keeper"
	stktypes "github.com/cosmos/cosmos-sdk/x/staking/types"

	"github.com/anon-org/anon/v4/app/keepers"
	"github.com/anon-org/anon/v4/app/upgrades"
	"github.com/anon-org/anon/v4/app/upgrades/epoching"
	costkkeeper "github.com/anon-org/anon/v4/x/costaking/keeper"
	costktypes "github.com/anon-org/anon/v4/x/costaking/types"
	epochingkeeper "github.com/anon-org/anon/v4/x/epoching/keeper"
)

const UpgradeName = "v4.3"

var Upgrade = upgrades.Upgrade{
	UpgradeName:          UpgradeName,
	CreateUpgradeHandler: CreateUpgradeHandler,
	StoreUpgrades: store.StoreUpgrades{
		Added:   []string{},
		Deleted: []string{},
	},
}

func CreateUpgradeHandler(mm *module.Manager, configurator module.Configurator, keepers *keepers.AppKeepers) upgradetypes.UpgradeHandler {
	return func(ctx context.Context, plan upgradetypes.Plan, fromVM module.VersionMap) (module.VersionMap, error) {
		// Run migrations before applying any other state changes.
		migrations, err := mm.RunMigrations(ctx, configurator, fromVM)
		if err != nil {
			return nil, err
		}

		// Validate epoch boundary using epoching keeper
		if err := epoching.ValidateEpochBoundary(ctx, keepers.EpochingKeeper); err != nil {
			return nil, fmt.Errorf("epoch boundary validation failed: %w", err)
		}

		costkStoreKey := keepers.GetKey(costktypes.StoreKey)
		if costkStoreKey == nil {
			return nil, errors.New("invalid costaking types store key")
		}
		coStkStoreService := runtime.NewKVStoreService(costkStoreKey)

		// Reset co-staker rewards tracker for ActiveNtk
		if err := ResetCoStakerRwdsTrackerActiveNtk(
			ctx,
			keepers.EncCfg.Codec,
			coStkStoreService,
			keepers.EpochingKeeper,
			keepers.StakingKeeper,
			keepers.CostakingKeeper,
		); err != nil {
			return nil, err
		}

		return migrations, nil
	}
}

// ResetCoStakerRwdsTrackerActiveNtk resets the ActiveNtk in costaker rewards tracker
// It recalculates ActiveNtk for all NTK stakers based on current delegations to active validators
func ResetCoStakerRwdsTrackerActiveNtk(
	ctx context.Context,
	cdc codec.BinaryCodec,
	costkStoreService corestoretypes.KVStoreService,
	epochingKeeper epochingkeeper.Keeper,
	stkKeeper *stkkeeper.Keeper,
	coStkKeeper costkkeeper.Keeper,
) error {
	sb := collections.NewSchemaBuilder(costkStoreService)
	rwdTrackers := collections.NewMap(
		sb,
		costktypes.CostakerRewardsTrackerKeyPrefix,
		"costaker_rewards_tracker",
		collections.BytesKey,
		codec.CollValue[costktypes.CostakerRewardsTracker](cdc),
	)

	// Zero out ActiveNtk in all existing rewards trackers and track previous values
	accsWithActiveNtk, err := zeroOutCoStakerRwdsActiveNtk(ctx, rwdTrackers)
	if err != nil {
		return err
	}

	params := coStkKeeper.GetParams(ctx)
	endedPeriod, err := coStkKeeper.IncrementRewardsPeriod(ctx)
	if err != nil {
		return err
	}

	// Recalculate ActiveNtk for all NTK stakers based on current delegations to active validators
	if err := updateNTKStakersRwdTracker(ctx, endedPeriod, rwdTrackers, accsWithActiveNtk, epochingKeeper, stkKeeper, coStkKeeper, params); err != nil {
		return err
	}

	totalScore, err := getTotalScore(ctx, rwdTrackers)
	if err != nil {
		return err
	}

	currentRwd, err := coStkKeeper.GetCurrentRewards(ctx)
	if err != nil {
		return err
	}

	currentRwd.TotalScore = totalScore
	if err := currentRwd.Validate(); err != nil {
		return err
	}

	return coStkKeeper.SetCurrentRewards(ctx, *currentRwd)
}

type ActiveNtkTracked struct {
	PreviousActiveNtk math.Int
	CurrentActiveNtk  math.Int
}

// updateNTKStakersRwdTracker retrieves all NTK stakers delegating to active validators and updates their ActiveNtk
func updateNTKStakersRwdTracker(
	ctx context.Context,
	period uint64,
	rwdTrackers collections.Map[[]byte, costktypes.CostakerRewardsTracker],
	accsWithActiveNtk map[string]ActiveNtkTracked,
	epochingKeeper epochingkeeper.Keeper,
	stkKeeper *stkkeeper.Keeper,
	coStkKeeper costkkeeper.Keeper,
	params costktypes.Params,
) error {
	// Get all NTK stakers delegating to active validators in the current epoch
	ntkStakers, err := getAllNTKStakers(ctx, epochingKeeper, stkKeeper)
	if err != nil {
		return fmt.Errorf("failed to get all NTK stakers: %w", err)
	}

	// Add current NTK delegations to the tracking map
	for delegatorAddr, ntkAmount := range ntkStakers {
		data, found := accsWithActiveNtk[delegatorAddr]
		if found {
			data.CurrentActiveNtk = data.CurrentActiveNtk.Add(ntkAmount)
			accsWithActiveNtk[delegatorAddr] = data
		} else {
			accsWithActiveNtk[delegatorAddr] = ActiveNtkTracked{
				PreviousActiveNtk: math.ZeroInt(),
				CurrentActiveNtk:  ntkAmount,
			}
		}
	}

	// Update the costaker rewards trackers for all accounts
	for accAddrStr, ntkTracked := range accsWithActiveNtk {
		if err := updateCostakerActiveNtkRewardsTracker(
			ctx,
			coStkKeeper,
			period,
			rwdTrackers,
			sdk.MustAccAddressFromBech32(accAddrStr),
			params,
			ntkTracked,
		); err != nil {
			return err
		}
	}

	return nil
}

// getAllNTKStakers retrieves all NTK stakers by iterating over the current epoch's active validators
func getAllNTKStakers(ctx context.Context, epochingKeeper epochingkeeper.Keeper, stkKeeper *stkkeeper.Keeper) (map[string]math.Int, error) {
	stkQuerier := stkkeeper.NewQuerier(stkKeeper)
	ntkStakers := make(map[string]math.Int)

	// Get the current epoch's validator set from epoching keeper
	valSet := epochingKeeper.GetCurrentValidatorSet(ctx)

	// Iterate over validators in the current epoch
	for _, val := range valSet {
		valAddr := sdk.ValAddress(val.Addr)

		// Get all delegations for this active validator
		if err := getValidatorDelegations(ctx, stkQuerier, valAddr.String(), ntkStakers); err != nil {
			return nil, fmt.Errorf("failed to get delegations for validator %s: %w", valAddr.String(), err)
		}
	}

	return ntkStakers, nil
}

// getValidatorDelegations gets all delegations for a specific validator
func getValidatorDelegations(ctx context.Context, stkQuerier stkkeeper.Querier, validatorAddr string, ntkStakers map[string]math.Int) error {
	var nextKey []byte

	for {
		req := &stktypes.QueryValidatorDelegationsRequest{
			ValidatorAddr: validatorAddr,
			Pagination: &query.PageRequest{
				Key: nextKey,
			},
		}

		res, err := stkQuerier.ValidatorDelegations(ctx, req)
		if err != nil {
			return err
		}

		for _, delegation := range res.DelegationResponses {
			delegatorAddr := delegation.Delegation.DelegatorAddress
			amount := delegation.Balance.Amount

			if existing, found := ntkStakers[delegatorAddr]; found {
				ntkStakers[delegatorAddr] = existing.Add(amount)
			} else {
				ntkStakers[delegatorAddr] = amount
			}
		}

		if res.Pagination == nil || len(res.Pagination.NextKey) == 0 {
			break
		}
		nextKey = res.Pagination.NextKey
	}

	return nil
}

// updateCostakerActiveNtkRewardsTracker creates or updates a costaker rewards tracker with ActiveNtk
func updateCostakerActiveNtkRewardsTracker(
	ctx context.Context,
	coStkKeeper costkkeeper.Keeper,
	endedPeriod uint64,
	rwdTrackers collections.Map[[]byte, costktypes.CostakerRewardsTracker],
	stakerAddr sdk.AccAddress,
	params costktypes.Params,
	ntkTracked ActiveNtkTracked,
) error {
	addrKey := []byte(stakerAddr)

	// Try to get existing tracker
	rt, err := rwdTrackers.Get(ctx, addrKey)
	if err != nil && !errors.Is(err, collections.ErrNotFound) {
		return err
	}

	if errors.Is(err, collections.ErrNotFound) {
		// This should not happen as we're updating existing trackers only
		return nil
	}

	// Update existing tracker (set the ActiveNtk because it was zeroed out before)
	// Update the StartPeriodCumulativeReward only if the ActiveNtk is changing
	rt.ActiveNtk = rt.ActiveNtk.Add(ntkTracked.CurrentActiveNtk)
	diff := ntkTracked.CurrentActiveNtk.Sub(ntkTracked.PreviousActiveNtk)
	if !diff.IsZero() { // if there is any diff, the period needs to be increased
		rt.StartPeriodCumulativeReward = endedPeriod
		if err := coStkKeeper.CalculateCostakerRewardsAndSendToGauge(ctx, stakerAddr, endedPeriod); err != nil {
			return err
		}
	}

	// Update score
	rt.UpdateScore(params.ScoreRatioBtcByNtk)

	// Save tracker
	if err := rwdTrackers.Set(ctx, addrKey, rt); err != nil {
		return err
	}

	return nil
}

// zeroOutCoStakerRwdsActiveNtk zeros out ActiveNtk in all costaker rewards trackers
func zeroOutCoStakerRwdsActiveNtk(
	ctx context.Context,
	rwdTrackers collections.Map[[]byte, costktypes.CostakerRewardsTracker],
) (map[string]ActiveNtkTracked, error) {
	accsWithActiveNtk := make(map[string]ActiveNtkTracked)
	iter, err := rwdTrackers.Iterate(ctx, nil)
	if err != nil {
		return accsWithActiveNtk, err
	}
	defer iter.Close()

	for ; iter.Valid(); iter.Next() {
		costakerAddr, err := iter.Key()
		if err != nil {
			return accsWithActiveNtk, err
		}

		tracker, err := iter.Value()
		if err != nil {
			return accsWithActiveNtk, err
		}
		if tracker.ActiveNtk.IsZero() {
			continue
		}

		sdkAddr := sdk.AccAddress(costakerAddr)
		accsWithActiveNtk[sdkAddr.String()] = ActiveNtkTracked{
			PreviousActiveNtk: tracker.ActiveNtk,
			CurrentActiveNtk:  math.ZeroInt(),
		}

		// Zero out ActiveNtk
		tracker.ActiveNtk = math.ZeroInt()
		if err := rwdTrackers.Set(ctx, costakerAddr, tracker); err != nil {
			return accsWithActiveNtk, err
		}
	}

	return accsWithActiveNtk, nil
}

func getTotalScore(
	ctx context.Context,
	rwdTrackers collections.Map[[]byte, costktypes.CostakerRewardsTracker],
) (math.Int, error) {
	totalScore := math.ZeroInt()
	iter, err := rwdTrackers.Iterate(ctx, nil)
	if err != nil {
		return totalScore, err
	}
	defer iter.Close()

	for ; iter.Valid(); iter.Next() {
		tracker, err := iter.Value()
		if err != nil {
			return totalScore, err
		}

		totalScore = totalScore.Add(tracker.TotalScore)
	}

	return totalScore, nil
}
