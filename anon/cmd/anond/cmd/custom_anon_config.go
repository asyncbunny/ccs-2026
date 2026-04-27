package cmd

import (
	"fmt"
	"path/filepath"
	"strconv"

	"github.com/anon-org/anon/v4/app/ante"
	"github.com/anon-org/anon/v4/app/signer"
	cmtcfg "github.com/cometbft/cometbft/config"

	wasmtypes "github.com/CosmWasm/wasmd/x/wasm/types"

	serverconfig "github.com/cosmos/cosmos-sdk/server/config"

	appparams "github.com/anon-org/anon/v4/app/params"
	anc "github.com/anon-org/anon/v4/types"
)

type BtcConfig struct {
	Network string `mapstructure:"network"`
}

func defaultAnonBtcConfig() BtcConfig {
	return BtcConfig{
		Network: string(anc.BtcMainnet),
	}
}

type BlsConfig struct {
	BlsKeyFile string `mapstructure:"bls-key-file"`
}

func defaultAnonBlsConfig() BlsConfig {
	return BlsConfig{
		BlsKeyFile: filepath.Join(cmtcfg.DefaultConfigDir, signer.DefaultBlsKeyName),
	}
}

type AnonMempoolConfig struct {
	MaxGasWantedPerTx string `mapstructure:"max-gas-wanted-per-tx"`
}

func defaultAnonMempoolConfig() AnonMempoolConfig {
	return AnonMempoolConfig{
		MaxGasWantedPerTx: strconv.Itoa(ante.DefaultMaxGasWantedPerTx),
	}
}

type AnonAppConfig struct {
	serverconfig.Config `mapstructure:",squash"`

	Wasm wasmtypes.NodeConfig `mapstructure:"wasm"`

	BtcConfig BtcConfig `mapstructure:"btc-config"`

	BlsConfig BlsConfig `mapstructure:"bls-config"`

	AnonMempoolConfig AnonMempoolConfig `mapstructure:"anon-mempool"`
}

func DefaultAnonAppConfig() *AnonAppConfig {
	baseConfig := *serverconfig.DefaultConfig()
	// Update the default Mempool.MaxTxs to be 0 to make sure the PriorityNonceMempool is used
	baseConfig.Mempool.MaxTxs = 0
	// The SDK's default minimum gas price is set to "0.002uanc" (empty value) inside
	// app.toml, in order to avoid spamming attacks due to transactions with 0 gas price.
	baseConfig.MinGasPrices = fmt.Sprintf("%f%s", appparams.GlobalMinGasPrice, appparams.BaseCoinUnit)
	return &AnonAppConfig{
		Config:               baseConfig,
		Wasm:                 wasmtypes.DefaultNodeConfig(),
		BtcConfig:            defaultAnonBtcConfig(),
		BlsConfig:            defaultAnonBlsConfig(),
		AnonMempoolConfig: defaultAnonMempoolConfig(),
	}
}

func DefaultAnonTemplate() string {
	return serverconfig.DefaultConfigTemplate + wasmtypes.DefaultConfigTemplate() + `
###############################################################################
###                        BLS configuration                                ###
###############################################################################

[bls-config]
# Path to the BLS key file (if empty, defaults to $HOME/.anond/config/bls_key.json)
bls-key-file = "{{ .BlsConfig.BlsKeyFile }}"

###############################################################################
###                      Anon Bitcoin configuration                      ###
###############################################################################

[btc-config]

# Configures which bitcoin network should be used for checkpointing
# valid values are: [mainnet, testnet, simnet, signet, regtest]
network = "{{ .BtcConfig.Network }}"

###############################################################################
###                      Anon Mempool Configuration                      ###
###############################################################################

[anon-mempool]
# This is the max allowed gas for any tx.
# This is only for local mempool purposes, and thus	is only ran on check tx.
max-gas-wanted-per-tx = "{{ .AnonMempoolConfig.MaxGasWantedPerTx }}"
`
}
