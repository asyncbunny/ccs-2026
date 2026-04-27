package reporter

import (
	"context"

	"cosmossdk.io/errors"
	"github.com/anon-org/anon/v4/client/anonclient"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/anon-org/anon/v4/client/config"
	btcctypes "github.com/anon-org/anon/v4/x/btccheckpoint/types"
	btclctypes "github.com/anon-org/anon/v4/x/btclightclient/types"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
)

type AnonClient interface {
	MustGetAddr() string
	GetConfig() *config.AnonConfig
	BTCCheckpointParams() (*btcctypes.QueryParamsResponse, error)
	InsertHeaders(ctx context.Context, msgs *btclctypes.MsgInsertHeaders) (*anonclient.RelayerTxResponse, error)
	ContainsBTCBlock(blockHash *chainhash.Hash) (*btclctypes.QueryContainsBytesResponse, error)
	BTCHeaderChainTip() (*btclctypes.QueryTipResponse, error)
	BTCBaseHeader() (*btclctypes.QueryBaseHeaderResponse, error)
	InsertBTCSpvProof(ctx context.Context, msg *btcctypes.MsgInsertBTCSpvProof) (*anonclient.RelayerTxResponse, error)
	ReliablySendMsg(ctx context.Context, msg sdk.Msg, expectedErrors []*errors.Error, unrecoverableErrors []*errors.Error) (*anonclient.RelayerTxResponse, error)
	Stop() error
}
