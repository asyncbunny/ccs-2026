package types // nolint:revive

import (
	"context"
	"fmt"
	"math/rand"
	"testing"

	"github.com/anon-org/anon/v4/testutil/datagen"
	"github.com/cosmos/cosmos-sdk/client/flags"
	"go.uber.org/zap"

	"cosmossdk.io/log"
	"github.com/anon-org/anon/v4/app"
	anccmd "github.com/anon-org/anon/v4/cmd/anond/cmd"
	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/server"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	genutiltest "github.com/cosmos/cosmos-sdk/x/genutil/client/testutil"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
)

func FuzzGetGenesisInfoFromFile(f *testing.F) {
	datagen.AddRandomSeedsToFuzzer(f, 10)
	f.Fuzz(func(t *testing.T, seed int64) {
		r := rand.New(rand.NewSource(seed))
		home := t.TempDir()
		logger := log.NewNopLogger()
		tmpAnon := app.NewTmpAnonApp()
		cfg, err := genutiltest.CreateDefaultCometConfig(home)
		require.NoError(t, err)

		err = genutiltest.ExecInitCmd(tmpAnon.BasicModuleManager, home, tmpAnon.AppCodec())
		require.NoError(t, err)

		serverCtx := server.NewContext(viper.New(), cfg, logger)
		clientCtx := client.Context{}.
			WithCodec(tmpAnon.AppCodec()).
			WithHomeDir(home).
			WithTxConfig(tmpAnon.TxConfig())

		ctx := t.Context()
		ctx = context.WithValue(ctx, server.ServerContextKey, serverCtx)
		ctx = context.WithValue(ctx, client.ClientContextKey, &clientCtx)
		cmd := anccmd.TestnetCmd(tmpAnon.BasicModuleManager, banktypes.GenesisBalancesIterator{})

		validatorNum := r.Intn(10) + 1
		epochInterval := r.Intn(500) + 2
		// Heiight must be difficulty adjustment block
		baseHeight := 0
		cmd.SetArgs([]string{
			fmt.Sprintf("--%s=test", flags.FlagKeyringBackend),
			fmt.Sprintf("--v=%v", validatorNum),
			fmt.Sprintf("--output-dir=%s", home),
			fmt.Sprintf("--btc-base-header-height=%v", baseHeight),
			fmt.Sprintf("--epoch-interval=%v", epochInterval),
		})
		err = cmd.ExecuteContext(ctx)
		require.NoError(t, err)

		genFile := cfg.GenesisFile()
		genesisInfo, err := GetGenesisInfoFromFile(genFile, zap.NewNop())
		require.NoError(t, err)
		require.Equal(t, uint64(epochInterval), genesisInfo.epochInterval)
		require.Len(t, genesisInfo.valSet.ValSet, validatorNum)
		require.Equal(t, uint32(baseHeight), genesisInfo.baseBTCHeight)
	})
}
