package monitor_test

import (
	"testing"

	"github.com/anon-org/anon/v4/x/monitor"
	"github.com/stretchr/testify/require"

	simapp "github.com/anon-org/anon/v4/app"
	"github.com/anon-org/anon/v4/x/monitor/types"
)

func TestExportGenesis(t *testing.T) {
	app := simapp.Setup(t, false)
	ctx := app.BaseApp.NewContext(false)
	genesisState := monitor.ExportGenesis(ctx, app.MonitorKeeper)
	require.Equal(t, genesisState, types.DefaultGenesis())
}
