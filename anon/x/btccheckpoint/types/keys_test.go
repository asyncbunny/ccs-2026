package types_test

import (
	"testing"

	"github.com/anon-org/anon/v4/testutil/store"
	"github.com/anon-org/anon/v4/x/btccheckpoint/types"
)

func TestNoKeyCollision(t *testing.T) {
	keys := map[string]interface{}{
		"SubmisionKeyPrefix":       types.SubmisionKeyPrefix,
		"EpochDataPrefix":          types.EpochDataPrefix,
		"LastFinalizedEpochKey":    types.LastFinalizedEpochKey,
		"BtcLightClientUpdatedKey": types.BtcLightClientUpdatedKey,
		"ParamsKey":                types.ParamsKey,
	}

	store.CheckKeyCollisions(t, keys)
}
