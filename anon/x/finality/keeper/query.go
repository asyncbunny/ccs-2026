package keeper

import (
	"github.com/anon-org/anon/v4/x/finality/types"
)

var _ types.QueryServer = Keeper{}
