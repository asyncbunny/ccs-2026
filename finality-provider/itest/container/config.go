package container

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/anon-org/finality-provider/testutil"
)

// ImageConfig contains all images and their respective tags
// needed for running e2e tests.
type ImageConfig struct {
	AnonRepository     string
	AnonVersion        string
	AnvilRepository       string
	AnvilVersion          string
	AnvilBlockTimeSeconds string // Block time in seconds for Anvil
}

const (
	dockerAnondRepository = "anon/anond"
	dockerAnvilRepository    = "ghcr.io/foundry-rs/foundry"
)

// NewImageConfig returns ImageConfig needed for running e2e test.
func NewImageConfig(t *testing.T) ImageConfig {
	// NOTE: currently there's no tag for the latest API breaking changes
	// on anon node. Because of this, we're using the commit hash instead of
	// the version tag. There's a docker image pushed to the registry with every PR
	// merged to main.
	// TODO on creation of the v1rc7 tag (or other useful tag for these tests), we should use the GetAnonVersion() back again
	anondVersion, err := testutil.GetAnonVersion()
	// anondVersion, err := testutil.GetAnonCommitHash()
	require.NoError(t, err)

	return ImageConfig{
		AnonRepository:     dockerAnondRepository,
		AnonVersion:        anondVersion,
		AnvilRepository:       dockerAnvilRepository,
		AnvilVersion:          "v1.2.3",
		AnvilBlockTimeSeconds: "1", // Default block time for Anvil
	}
}
