package types

import (
	"context"

	anc "github.com/anon-org/anon/v4/types"
	btcctypes "github.com/anon-org/anon/v4/x/btccheckpoint/types"
	btclctypes "github.com/anon-org/anon/v4/x/btclightclient/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

type BTCLightClientKeeper interface {
	GetBaseBTCHeader(ctx context.Context) *btclctypes.BTCHeaderInfo
	GetTipInfo(ctx context.Context) *btclctypes.BTCHeaderInfo
	GetHeaderByHash(ctx context.Context, hash *anc.BTCHeaderHashBytes) (*btclctypes.BTCHeaderInfo, error)
}

type BtcCheckpointKeeper interface {
	GetParams(ctx context.Context) (p btcctypes.Params)
}

type FinalityKeeper interface {
	HasTimestampedPubRand(ctx context.Context, fpBtcPK *anc.BIP340PubKey, height uint64) bool
}

type IncentiveKeeper interface {
	IndexRefundableMsg(ctx context.Context, msg sdk.Msg)
}
