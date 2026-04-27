package keeper_test

import (
	"math/rand"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"

	"github.com/anon-org/anon/v4/testutil/datagen"
	keepertest "github.com/anon-org/anon/v4/testutil/keeper"
	btclctypes "github.com/anon-org/anon/v4/x/btclightclient/types"
	"github.com/anon-org/anon/v4/x/btcstaking/types"
)

func FuzzBTCHeightIndex(f *testing.F) {
	datagen.AddRandomSeedsToFuzzer(f, 10)

	f.Fuzz(func(t *testing.T, seed int64) {
		r := rand.New(rand.NewSource(seed))
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		// mock BTC light client
		btclcKeeper := types.NewMockBTCLightClientKeeper(ctrl)
		keeper, ctx := keepertest.BTCStakingKeeper(t, btclcKeeper, nil, nil)

		// randomise Anon height and BTC height
		anonHeight := datagen.RandomInt(r, 100)
		ctx = datagen.WithCtxHeight(ctx, anonHeight)
		btcHeight := uint32(datagen.RandomInt(r, 100))
		btclcKeeper.EXPECT().GetTipInfo(gomock.Any()).Return(&btclctypes.BTCHeaderInfo{Height: btcHeight}).Times(1)
		keeper.IndexBTCHeight(ctx)

		// assert BTC height
		actualBtcHeight := keeper.GetBTCHeightAtAnonHeight(ctx, anonHeight)
		require.Equal(t, btcHeight, actualBtcHeight)
		// assert current BTC height
		curBtcHeight := keeper.GetCurrentBTCHeight(ctx)
		require.Equal(t, btcHeight, curBtcHeight)
	})
}
