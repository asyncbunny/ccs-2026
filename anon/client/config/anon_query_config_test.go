package config_test

import (
	"testing"

	"github.com/anon-org/anon/v4/client/config"
	"github.com/stretchr/testify/require"
)

// TestAnonQueryConfig ensures that the default Anon query config is valid
func TestAnonQueryConfig(t *testing.T) {
	defaultConfig := config.DefaultAnonQueryConfig()
	err := defaultConfig.Validate()
	require.NoError(t, err)
}
