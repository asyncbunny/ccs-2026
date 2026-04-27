package poller

import (
	checkpointingtypes "github.com/anon-org/anon/v4/x/checkpointing/types"
	sdkquerytypes "github.com/cosmos/cosmos-sdk/types/query"
)

type AnonQueryClient interface {
	RawCheckpointList(status checkpointingtypes.CheckpointStatus, pagination *sdkquerytypes.PageRequest) (*checkpointingtypes.QueryRawCheckpointListResponse, error)
}
