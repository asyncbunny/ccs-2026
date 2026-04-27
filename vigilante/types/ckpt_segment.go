package types // nolint:revive

import (
	"time"

	"github.com/anon-org/anon/v4/btctxformatter"
	btcctypes "github.com/anon-org/anon/v4/x/btccheckpoint/types"
	"github.com/btcsuite/btcd/btcutil"
)

// CkptSegment is a segment of the Anon checkpoint, including
// - Data: actual OP_RETURN data excluding the Anon header
// - Index: index of the segment in the checkpoint
// - TxIdx: index of the tx in AssocBlock
// - AssocBlock: pointer to the block that contains the tx that carries the ckpt segment
type CkptSegment struct {
	*btctxformatter.AnonData
	TxIdx      int
	AssocBlock *IndexedBlock
	Timestamp  time.Time
}

func NewCkptSegment(tag btctxformatter.AnonTag, version btctxformatter.FormatVersion, block *IndexedBlock, tx *btcutil.Tx) *CkptSegment {
	opReturnData, err := btcctypes.ExtractStandardOpReturnData(tx)
	if err != nil {
		return nil
	}
	ancData, err := btctxformatter.IsAnonCheckpointData(tag, version, opReturnData)
	if err != nil {
		return nil
	}

	return &CkptSegment{
		AnonData: ancData,
		TxIdx:       tx.Index(),
		AssocBlock:  block,
	}
}
