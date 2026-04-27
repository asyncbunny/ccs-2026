package poller_test

import (
	"math/rand"
	"sort"
	"sync"
	"testing"

	"github.com/anon-org/anon/v4/testutil/datagen"
	checkpointingtypes "github.com/anon-org/anon/v4/x/checkpointing/types"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"

	"github.com/anon-org/vigilante/submitter/poller"
)

func FuzzPollingCheckpoints(f *testing.F) {
	/*
		Checks:
		- the poller polls Sealed checkpoints,
		only the oldest one being pushed into the channel

		Data generation:
		- a series of raw checkpoints
	*/
	datagen.AddRandomSeedsToFuzzer(f, 10)
	f.Fuzz(func(t *testing.T, seed int64) {
		t.Parallel()
		r := rand.New(rand.NewSource(seed))

		var wg sync.WaitGroup
		n := r.Intn(10) + 1
		sealedCkpts := make([]*checkpointingtypes.RawCheckpointWithMetaResponse, n)
		for i := 0; i < n; i++ {
			ckpt := datagen.GenRandomRawCheckpointWithMeta(r)
			ckpt.Status = checkpointingtypes.Sealed
			sealedCkpts[i] = ckpt.ToResponse()
		}
		sort.Slice(sealedCkpts, func(i, j int) bool {
			return sealedCkpts[i].Ckpt.EpochNum < sealedCkpts[j].Ckpt.EpochNum
		})
		ancClient := poller.NewMockAnonQueryClient(gomock.NewController(t))
		ancClient.EXPECT().RawCheckpointList(gomock.Eq(checkpointingtypes.Sealed), gomock.Nil()).Return(
			&checkpointingtypes.QueryRawCheckpointListResponse{RawCheckpoints: sealedCkpts}, nil)
		testPoller := poller.New(ancClient, 10)
		wg.Add(1)
		var ckpt *checkpointingtypes.RawCheckpointWithMetaResponse
		go func() {
			defer wg.Done()
			ckpt = <-testPoller.GetSealedCheckpointChan()
		}()
		err := testPoller.PollSealedCheckpoints()
		wg.Wait()
		require.NoError(t, err)
		require.Equal(t, sealedCkpts[0], ckpt)
	})
}
