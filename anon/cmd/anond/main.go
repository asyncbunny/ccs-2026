package main

import (
	"os"

	"cosmossdk.io/log"

	"github.com/anon-org/anon/v4/app"
	"github.com/anon-org/anon/v4/cmd/anond/cmd"
	svrcmd "github.com/cosmos/cosmos-sdk/server/cmd"

	"github.com/anon-org/anon/v4/app/params"
)

func main() {
	params.SetAddressPrefixes()
	rootCmd := cmd.NewRootCmd()

	if err := svrcmd.Execute(rootCmd, app.AnonAppEnvPrefix, app.DefaultNodeHome); err != nil {
		log.NewLogger(rootCmd.OutOrStderr()).Error("failure when running app", "err", err)
		os.Exit(1)
	}
}
