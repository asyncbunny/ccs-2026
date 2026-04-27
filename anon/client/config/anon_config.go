package config

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"time"

	"github.com/anon-org/anon/v4/client/anonclient"
)

// AnonConfig defines configuration for the Anon client
// adapted from https://github.com/strangelove-ventures/lens/blob/v0.5.1/client/config.go
type AnonConfig struct {
	Key              string        `mapstructure:"key"`
	ChainID          string        `mapstructure:"chain-id"`
	RPCAddr          string        `mapstructure:"rpc-addr"`
	GRPCAddr         string        `mapstructure:"grpc-addr"`
	AccountPrefix    string        `mapstructure:"account-prefix"`
	KeyringBackend   string        `mapstructure:"keyring-backend"`
	GasAdjustment    float64       `mapstructure:"gas-adjustment"`
	GasPrices        string        `mapstructure:"gas-prices"`
	KeyDirectory     string        `mapstructure:"key-directory"`
	Debug            bool          `mapstructure:"debug"`
	Timeout          time.Duration `mapstructure:"timeout"`
	BlockTimeout     time.Duration `mapstructure:"block-timeout"`
	OutputFormat     string        `mapstructure:"output-format"`
	SignModeStr      string        `mapstructure:"sign-mode"`
	SubmitterAddress string        `mapstructure:"submitter-address"`
}

func (cfg *AnonConfig) Validate() error {
	if _, err := url.Parse(cfg.RPCAddr); err != nil {
		return fmt.Errorf("rpc-addr is not correctly formatted: %w", err)
	}
	if cfg.Timeout <= 0 {
		return fmt.Errorf("timeout must be positive")
	}
	if cfg.BlockTimeout <= 0 {
		return fmt.Errorf("block-timeout must be positive")
	}
	return nil
}

func (cfg *AnonConfig) ToCosmosProviderConfig() anonclient.CosmosProviderConfig {
	return anonclient.CosmosProviderConfig{
		Key:            cfg.Key,
		ChainID:        cfg.ChainID,
		RPCAddr:        cfg.RPCAddr,
		AccountPrefix:  cfg.AccountPrefix,
		KeyringBackend: cfg.KeyringBackend,
		GasAdjustment:  cfg.GasAdjustment,
		GasPrices:      cfg.GasPrices,
		KeyDirectory:   cfg.KeyDirectory,
		Debug:          cfg.Debug,
		Timeout:        cfg.Timeout.String(),
		BlockTimeout:   cfg.BlockTimeout.String(),
		OutputFormat:   cfg.OutputFormat,
		SignModeStr:    cfg.SignModeStr,
	}
}

func DefaultAnonConfig() AnonConfig {
	return AnonConfig{
		Key:     "node0",
		ChainID: "chain-test",
		// see https://docs.cosmos.network/master/core/grpc_rest.html for default ports
		// TODO: configure HTTPS for Anon's RPC server
		// TODO: how to use Cosmos SDK's RPC server (port 1317) rather than Tendermint's RPC server (port 26657)?
		RPCAddr: "http://localhost:26657",
		// TODO: how to support GRPC in the Anon client?
		GRPCAddr:         "https://localhost:9090",
		AccountPrefix:    "anc",
		KeyringBackend:   "test",
		GasAdjustment:    1.2,
		GasPrices:        "0.01uanc",
		KeyDirectory:     defaultAnonHome(),
		Debug:            true,
		Timeout:          20 * time.Second,
		BlockTimeout:     10 * time.Minute,
		OutputFormat:     "json",
		SignModeStr:      "direct",
		SubmitterAddress: "anc1v6k7k9s8md3k29cu9runasstq5zaa0lpznk27w", // this is currently a placeholder, will not recognized by Anon
	}
}

// defaultAnonHome returns the default Anon node directory, which is $HOME/.anond
// copied from #
func defaultAnonHome() string {
	userHomeDir, err := os.UserHomeDir()
	if err != nil {
		panic(err)
	}

	return filepath.Join(userHomeDir, ".anond")
}
