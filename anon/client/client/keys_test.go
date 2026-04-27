package client_test

import (
	"math/rand"
	"strings"
	"testing"

	anc "github.com/anon-org/anon/v4/app"
	"github.com/anon-org/anon/v4/client/client"
	"github.com/anon-org/anon/v4/client/config"
	"github.com/anon-org/anon/v4/testutil/datagen"
	"github.com/cosmos/cosmos-sdk/crypto/hd"
	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	"github.com/stretchr/testify/require"
)

func FuzzKeys(f *testing.F) {
	datagen.AddRandomSeedsToFuzzer(f, 10)

	f.Fuzz(func(t *testing.T, seed int64) {
		r := rand.New(rand.NewSource(seed))

		// create a keyring
		keyringName := datagen.GenRandomHexStr(r, 10)
		dir := t.TempDir()
		mockIn := strings.NewReader("")
		cdc := anc.GetEncodingConfig()
		kr, err := keyring.New(keyringName, "test", dir, mockIn, cdc.Codec)
		require.NoError(t, err)

		// create a random key pair in this keyring
		keyName := datagen.GenRandomHexStr(r, 10)
		_, _, err = kr.NewMnemonic(
			keyName,
			keyring.English,
			hd.CreateHDPath(118, 0, 0).String(),
			keyring.DefaultBIP39Passphrase,
			hd.Secp256k1,
		)
		require.NoError(t, err)

		// create a Anon client with this random keyring
		cfg := config.DefaultAnonConfig()
		cfg.KeyDirectory = dir
		cfg.Key = keyName
		cl, err := client.New(&cfg, nil)
		require.NoError(t, err)

		// retrieve the key info from key ring
		keys, err := kr.List()
		require.NoError(t, err)
		require.Equal(t, 1, len(keys))

		// test if the key is consistent in Anon client and keyring
		ancAddr := cl.MustGetAddr()
		addr, _ := keys[0].GetAddress()
		require.Equal(t, addr.String(), ancAddr)
	})
}
