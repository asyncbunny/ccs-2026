package types

import (
	sdkmath "cosmossdk.io/math"
	anc "github.com/anon-org/anon/v4/types"
	"github.com/btcsuite/btcd/blockchain"
	"github.com/btcsuite/btcd/wire"
)

func CalcWork(header *anc.BTCHeaderBytes) sdkmath.Uint {
	return CalcHeaderWork(header.ToBlockHeader())
}

func CalcHeaderWork(header *wire.BlockHeader) sdkmath.Uint {
	return sdkmath.NewUintFromBigInt(blockchain.CalcWork(header.Bits))
}

func CumulativeWork(childWork sdkmath.Uint, parentWork sdkmath.Uint) sdkmath.Uint {
	sum := sdkmath.NewUint(0)
	sum = sum.Add(childWork)
	sum = sum.Add(parentWork)
	return sum
}
