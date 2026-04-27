package finality_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	keepertest "github.com/anon-org/anon/v4/testutil/keeper"
	"github.com/anon-org/anon/v4/testutil/nullify"
	"github.com/anon-org/anon/v4/x/finality"
	"github.com/anon-org/anon/v4/x/finality/types"
)

func TestGenesis(t *testing.T) {
	genesisState := types.GenesisState{
		Params: types.DefaultParams(),
	}

	k, ctx := keepertest.FinalityKeeper(t, nil, nil, nil, nil)
	finality.InitGenesis(ctx, *k, genesisState)
	got := finality.ExportGenesis(ctx, *k)
	require.NotNil(t, got)

	nullify.Fill(&genesisState)
	nullify.Fill(got)
}
