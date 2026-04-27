package store_test

import (
	"math/rand"
	"testing"

	ancdatagen "github.com/anon-org/anon/v4/testutil/datagen"
	"github.com/anon-org/vigilante/monitor/store"
	"github.com/anon-org/vigilante/testutil"
	"github.com/stretchr/testify/require"
)

func TestEmptyStore(t *testing.T) {
	t.Parallel()
	db := testutil.MakeTestBackend(t)
	s, err := store.NewMonitorStore(db)
	require.NoError(t, err)

	_, exists, err := s.LatestEpoch()
	require.NoError(t, err)
	require.False(t, exists)

	_, exists, err = s.LatestHeight()
	require.NoError(t, err)
	require.False(t, exists)
}

func FuzzStoringEpochs(f *testing.F) {
	// only 3 seeds as this is pretty slow test opening/closing db
	ancdatagen.AddRandomSeedsToFuzzer(f, 3)

	f.Fuzz(func(t *testing.T, seed int64) {
		t.Parallel()
		r := rand.New(rand.NewSource(seed))
		db := testutil.MakeTestBackend(t)
		s, err := store.NewMonitorStore(db)
		require.NoError(t, err)

		_, exists, err := s.LatestEpoch()
		require.NoError(t, err)
		require.False(t, exists)

		epoch := uint64(r.Int63n(1000) + 1)
		err = s.PutLatestEpoch(epoch)
		require.NoError(t, err)

		storedLatestEpoch, exists, err := s.LatestEpoch()
		require.NoError(t, err)
		require.True(t, exists)
		require.Equal(t, epoch, storedLatestEpoch)
	})
}

func FuzzStoringHeights(f *testing.F) {
	ancdatagen.AddRandomSeedsToFuzzer(f, 3)

	f.Fuzz(func(t *testing.T, seed int64) {
		t.Parallel()
		r := rand.New(rand.NewSource(seed))
		db := testutil.MakeTestBackend(t)
		s, err := store.NewMonitorStore(db)
		require.NoError(t, err)

		_, exists, err := s.LatestHeight()
		require.NoError(t, err)
		require.False(t, exists)

		height := uint64(r.Int63n(1000) + 1)
		err = s.PutLatestHeight(height)
		require.NoError(t, err)

		storedHeight, exists, err := s.LatestHeight()
		require.NoError(t, err)
		require.True(t, exists)
		require.Equal(t, height, storedHeight)
	})
}
