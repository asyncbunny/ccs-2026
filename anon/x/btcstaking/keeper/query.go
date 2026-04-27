package keeper

import (
	"github.com/anon-org/anon/v4/x/btcstaking/types"
)

var _ types.QueryServer = Keeper{}
