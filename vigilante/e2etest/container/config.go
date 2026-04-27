package container

import (
	"github.com/anon-org/vigilante/testutil"
	"github.com/stretchr/testify/require"
	"testing"
)

// ImageConfig contains all images and their respective tags
// needed for running e2e tests.
type ImageConfig struct {
	BitcoindRepository string
	BitcoindVersion    string
	AnonRepository  string
	AnonVersion     string
	ElectrsRepository  string
	ElectrsVersion     string
}

//nolint:deadcode
const (
	dockerBitcoindRepository = "lncm/bitcoind"
	dockerBitcoindVersionTag = "v28.0"
	dockerAnondRepository = "anon/anond"
	dockerElectrsRepository  = "mempool/electrs"
	dockerElectrsVersionTag  = "v3.1.0"
)

// NewImageConfig returns ImageConfig needed for running e2e test.
func NewImageConfig(t *testing.T) ImageConfig {
	anonVersion, err := testutil.GetAnonVersion()
	require.NoError(t, err)

	return ImageConfig{
		BitcoindRepository: dockerBitcoindRepository,
		BitcoindVersion:    dockerBitcoindVersionTag,
		AnonRepository:  dockerAnondRepository,
		AnonVersion:     anonVersion,
		ElectrsRepository:  dockerElectrsRepository,
		ElectrsVersion:     dockerElectrsVersionTag,
	}
}
