package submitter

import (
	"github.com/anon-org/vigilante/submitter/poller"

	btcctypes "github.com/anon-org/anon/v4/x/btccheckpoint/types"
)

type AnonQueryClient interface {
	poller.AnonQueryClient
	BTCCheckpointParams() (*btcctypes.QueryParamsResponse, error)
}
