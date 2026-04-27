package keeper_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	testkeeper "github.com/anon-org/anon/v4/testutil/keeper"
	"github.com/anon-org/anon/v4/x/finality/types"
)

func TestGetParams(t *testing.T) {
	k, ctx := testkeeper.FinalityKeeper(t, nil, nil, nil, nil)
	params := types.DefaultParams()

	err := k.SetParams(ctx, params)
	require.NoError(t, err)

	require.EqualValues(t, params, k.GetParams(ctx))
}
