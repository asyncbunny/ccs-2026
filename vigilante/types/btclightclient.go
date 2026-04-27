package types // nolint:revive

import (
	anontypes "github.com/anon-org/anon/v4/types"
	btcltypes "github.com/anon-org/anon/v4/x/btclightclient/types"
)

func NewMsgInsertHeaders(
	signer string,
	indexedBlocks []*IndexedBlock,
) *btcltypes.MsgInsertHeaders {
	headerBytes := make([]anontypes.BTCHeaderBytes, len(indexedBlocks))
	for i, ib := range indexedBlocks {
		headerBytes[i] = anontypes.NewBTCHeaderBytesFromBlockHeader(ib.Header)
	}

	return &btcltypes.MsgInsertHeaders{
		Signer:  signer,
		Headers: headerBytes,
	}
}
