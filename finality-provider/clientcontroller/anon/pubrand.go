package anon

import (
	"fmt"
	"github.com/anon-org/finality-provider/types"
)

var _ types.PubRandCommit = (*AnonPubRandCommit)(nil)

// AnonPubRandCommit represents the Anon-specific public randomness commitment response
//
//nolint:revive
type AnonPubRandCommit struct {
	StartHeight uint64 `json:"start_height"`
	NumPubRand  uint64 `json:"num_pub_rand"`
	EpochNum    uint64 `json:"epoch_num"`
	Commitment  []byte `json:"commitment"`
}

func (b *AnonPubRandCommit) GetStartHeight() uint64 {
	return b.StartHeight
}

func (b *AnonPubRandCommit) GetNumPubRand() uint64 {
	return b.NumPubRand
}

func (b *AnonPubRandCommit) GetCommitment() []byte {
	return b.Commitment
}

func (b *AnonPubRandCommit) GetEndHeight() uint64 { return b.StartHeight + b.NumPubRand - 1 }

func (b *AnonPubRandCommit) Validate() error {
	if b.NumPubRand < 1 {
		return fmt.Errorf("NumPubRand must be >= 1, got %d", b.NumPubRand)
	}

	return nil
}
