package keeper

import (
	"context"

	sdk "github.com/cosmos/cosmos-sdk/types"

	ftypes "github.com/anon-org/anon/v4/x/finality/types"
)

var _ ftypes.FinalityHooks = Hooks{}

// Wrapper struct
type Hooks struct {
	k Keeper
}

// Create new distribution hooks
func (k Keeper) Hooks() Hooks {
	return Hooks{k}
}

// AfterBtcDelegationUnbonded implements the FinalityHooks interface
// It handles the unbonding of a BTC delegation by removing the staked satoshis
// from the reward tracking system
func (h Hooks) AfterBtcDelegationUnbonded(ctx context.Context, fpAddr, btcDelAddr sdk.AccAddress, isFpActiveInPrevSet, isFpActiveInCurrSet bool, sats uint64) error {
	// All FPs are Anon FPs now, so always add to event tracking
	height := uint64(sdk.UnwrapSDKContext(ctx).HeaderInfo().Height)
	return h.k.AddEventBtcDelegationUnbonded(ctx, height, fpAddr, btcDelAddr, sats)
}

// AfterBtcDelegationActivated implements the FinalityHooks interface
// It handles the activation of a BTC delegation by adding the staked satoshis
// to the reward tracking system
func (h Hooks) AfterBtcDelegationActivated(ctx context.Context, fpAddr, btcDelAddr sdk.AccAddress, isFpActiveInPrevSet, isFpActiveInCurrSet bool, sats uint64) error {
	// All FPs are Anon FPs now, so always add to event tracking
	height := uint64(sdk.UnwrapSDKContext(ctx).HeaderInfo().Height)
	return h.k.AddEventBtcDelegationActivated(ctx, height, fpAddr, btcDelAddr, sats)
}

// AfterAncFpEntersActiveSet implements the FinalityHooks interface
func (h Hooks) AfterAncFpEntersActiveSet(ctx context.Context, fpAddr sdk.AccAddress) error {
	return nil
}

// AfterAncFpRemovedFromActiveSet implements the FinalityHooks interface
func (h Hooks) AfterAncFpRemovedFromActiveSet(ctx context.Context, fpAddr sdk.AccAddress) error {
	return nil
}
