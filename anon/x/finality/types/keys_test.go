package types_test

import (
	"testing"

	"github.com/anon-org/anon/v4/testutil/store"
	"github.com/anon-org/anon/v4/x/finality/types"
)

func TestNoKeyCollision(t *testing.T) {
	keys := map[string]interface{}{
		"BlockKey":                                   types.BlockKey,
		"VoteKey":                                    types.VoteKey,
		"PubRandKey":                                 types.PubRandKey,
		"PubRandCommitKey":                           types.PubRandCommitKey,
		"ParamsKey":                                  types.ParamsKey,
		"EvidenceKey":                                types.EvidenceKey,
		"NextHeightToFinalizeKey":                    types.NextHeightToFinalizeKey,
		"FinalityProviderSigningInfoKeyPrefix":       types.FinalityProviderSigningInfoKeyPrefix,
		"FinalityProviderMissedBlockBitmapKeyPrefix": types.FinalityProviderMissedBlockBitmapKeyPrefix,
		"VotingPowerKey":                             types.VotingPowerKey,
		"VotingPowerDistCacheKey":                    types.VotingPowerDistCacheKey,
		"NextHeightToRewardKey":                      types.NextHeightToRewardKey,
		"PubRandCommitIndexKeyPrefix":                types.PubRandCommitIndexKeyPrefix,
	}

	store.CheckKeyCollisions(t, keys)
}
