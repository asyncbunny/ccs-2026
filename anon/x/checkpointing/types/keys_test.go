package types_test

import (
	"testing"

	"github.com/anon-org/anon/v4/testutil/store"
	"github.com/anon-org/anon/v4/x/checkpointing/types"
)

func TestNoKeyCollision(t *testing.T) {
	keys := map[string]interface{}{
		"ValidatorBlsKeySetPrefix":         types.ValidatorBlsKeySetPrefix,
		"CkptsObjectPrefix":                types.CkptsObjectPrefix,
		"AddrToBlsKeyPrefix":               types.AddrToBlsKeyPrefix,
		"BlsKeyToAddrPrefix":               types.BlsKeyToAddrPrefix,
		"LastFinalizedEpochKey":            types.LastFinalizedEpochKey,
		"ConflictingCheckpointReceivedKey": types.ConflictingCheckpointReceivedKey,
	}

	store.CheckKeyCollisions(t, keys)
}
