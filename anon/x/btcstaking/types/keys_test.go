package types_test

import (
	"testing"

	"github.com/anon-org/anon/v4/testutil/store"
	"github.com/anon-org/anon/v4/x/btcstaking/types"
)

func TestNoKeyCollision(t *testing.T) {
	keys := map[string]interface{}{
		"ParamsKey":                   types.ParamsKey,
		"FinalityProviderKey":         types.FinalityProviderKey,
		"BTCDelegatorKey":             types.BTCDelegatorKey,
		"BTCDelegationKey":            types.BTCDelegationKey,
		"BTCHeightKey":                types.BTCHeightKey,
		"PowerDistUpdateKey":          types.PowerDistUpdateKey,
		"AllowedStakingTxHashesKey":   types.AllowedStakingTxHashesKey,
		"HeightToVersionMapKey":       types.HeightToVersionMapKey,
		"LargestBtcReorgInBlocks":     types.LargestBtcReorgInBlocks,
		"FpAncAddrKey":                types.FpAncAddrKey,
		"FinalityProvidersDeleted":    types.FinalityProvidersDeleted,
	}

	store.CheckKeyCollisions(t, keys)
}
