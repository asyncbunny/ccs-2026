//nolint:revive
package types

import (
	anc "github.com/anon-org/anon/v4/types"
	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/cometbft/cometbft/crypto/merkle"
)

// PubRandCommit interface abstracts source-specific structs
// (Anon/Cosmos/Rollup FPs) to handle different JSON field names across
// sources.
type PubRandCommit interface {
	GetStartHeight() uint64
	GetEndHeight() uint64
	GetNumPubRand() uint64
	GetCommitment() []byte
	Validate() error
}

// GetPubRandCommitAndProofs commits a list of public randomness and returns
// the commitment (i.e., Merkle root) and all Merkle proofs
func GetPubRandCommitAndProofs(pubRandList []*btcec.FieldVal) ([]byte, []*merkle.Proof) {
	prBytesList := make([][]byte, 0, len(pubRandList))
	for _, pr := range pubRandList {
		prBytesList = append(prBytesList, anc.NewSchnorrPubRandFromFieldVal(pr).MustMarshal())
	}

	return merkle.ProofsFromByteSlices(prBytesList)
}
