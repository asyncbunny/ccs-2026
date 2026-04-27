package atomicslasher

import (
	"context"

	"github.com/anon-org/anon/v4/client/anonclient"

	"cosmossdk.io/errors"
	bstypes "github.com/anon-org/anon/v4/x/btcstaking/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkquerytypes "github.com/cosmos/cosmos-sdk/types/query"
)

type AnonClient interface {
	FinalityProvider(fpBtcPkHex string) (*bstypes.QueryFinalityProviderResponse, error)
	BTCDelegations(status bstypes.BTCDelegationStatus, pagination *sdkquerytypes.PageRequest) (*bstypes.QueryBTCDelegationsResponse, error)
	BTCDelegation(stakingTxHashHex string) (*bstypes.QueryBTCDelegationResponse, error)
	BTCStakingParamsByVersion(version uint32) (*bstypes.QueryParamsByVersionResponse, error)
	ReliablySendMsg(ctx context.Context, msg sdk.Msg, expectedErrors []*errors.Error, unrecoverableErrors []*errors.Error) (*anonclient.RelayerTxResponse, error)
	MustGetAddr() string
}
