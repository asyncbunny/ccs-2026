//nolint:revive
package types

import (
	anctypes "github.com/anon-org/anon/v4/types"
	"github.com/anon-org/finality-provider/finality-provider/proto"
	"github.com/btcsuite/btcd/btcec/v2"
)

type FinalityProviderState interface {
	GetBtcPk() *btcec.PublicKey
	GetBtcPkBIP340() *anctypes.BIP340PubKey
	GetBtcPkHex() string
	GetChainID() []byte
	GetLastVotedHeight() uint64
	SetLastVotedHeight(height uint64) error
	GetStatus() proto.FinalityProviderStatus
	SetStatus(status proto.FinalityProviderStatus) error
}
