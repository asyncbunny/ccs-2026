package main

import (
	"fmt"
	"os"

	"github.com/anon-org/anon/v4/app/params"
	"github.com/anon-org/vigilante/cmd/vigilante/cmd"
)

// TODO: init log

func main() {
	params.SetAddressPrefixes()

	rootCmd := cmd.NewRootCmd()

	if err := rootCmd.Execute(); err != nil {
		fmt.Print(err.Error())
		os.Exit(1)
	}
}
