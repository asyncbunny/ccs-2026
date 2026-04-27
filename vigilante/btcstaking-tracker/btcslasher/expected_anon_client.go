package btcslasher

import (
	btcctypes "github.com/anon-org/anon/v4/x/btccheckpoint/types"
	bstypes "github.com/anon-org/anon/v4/x/btcstaking/types"
	ftypes "github.com/anon-org/anon/v4/x/finality/types"
	coretypes "github.com/cometbft/cometbft/rpc/core/types"
	"github.com/cosmos/cosmos-sdk/types/query"
)

type AnonQueryClient interface {
	BTCCheckpointParams() (*btcctypes.QueryParamsResponse, error)
	BTCStakingParamsByVersion(version uint32) (*bstypes.QueryParamsByVersionResponse, error)
	FinalityProviderDelegations(fpBTCPKHex string, pagination *query.PageRequest) (*bstypes.QueryFinalityProviderDelegationsResponse, error)
	ListEvidences(startHeight uint64, pagination *query.PageRequest) (*ftypes.QueryListEvidencesResponse, error)
	Subscribe(subscriber, query string, outCapacity ...int) (out <-chan coretypes.ResultEvent, err error)
	UnsubscribeAll(subscriber string) error
	IsRunning() bool
}
