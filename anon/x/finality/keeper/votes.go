package keeper

import (
	"context"
	"fmt"

	"github.com/cosmos/cosmos-sdk/runtime"

	"cosmossdk.io/store/prefix"
	anc "github.com/anon-org/anon/v4/types"
	"github.com/anon-org/anon/v4/x/finality/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

func (k Keeper) SetSig(ctx context.Context, height uint64, fpBtcPK *anc.BIP340PubKey, sig *anc.SchnorrEOTSSig) {
	store := k.voteHeightStore(ctx, height)
	store.Set(fpBtcPK.MustMarshal(), sig.MustMarshal())
}

func (k Keeper) HasSig(ctx context.Context, height uint64, fpBtcPK *anc.BIP340PubKey) bool {
	store := k.voteHeightStore(ctx, height)
	return store.Has(fpBtcPK.MustMarshal())
}

func (k Keeper) GetSig(ctx context.Context, height uint64, fpBtcPK *anc.BIP340PubKey) (*anc.SchnorrEOTSSig, error) {
	if uint64(sdk.UnwrapSDKContext(ctx).HeaderInfo().Height) < height {
		return nil, types.ErrHeightTooHigh
	}
	store := k.voteHeightStore(ctx, height)
	sigBytes := store.Get(fpBtcPK.MustMarshal())
	if len(sigBytes) == 0 {
		return nil, types.ErrVoteNotFound
	}
	sig, err := anc.NewSchnorrEOTSSig(sigBytes)
	if err != nil {
		panic(fmt.Errorf("failed to unmarshal EOTS signature: %w", err))
	}
	return sig, nil
}

// GetSigSet gets all EOTS signatures at a given height
func (k Keeper) GetSigSet(ctx context.Context, height uint64) map[string]*anc.SchnorrEOTSSig {
	store := k.voteHeightStore(ctx, height)
	iter := store.Iterator(nil, nil)
	defer iter.Close()

	// if there is no vote on this height, return nil
	if !iter.Valid() {
		return nil
	}

	sigs := map[string]*anc.SchnorrEOTSSig{}
	for ; iter.Valid(); iter.Next() {
		fpBTCPK, err := anc.NewBIP340PubKey(iter.Key())
		if err != nil {
			// failing to unmarshal finality provider's BTC PK in KVStore is a programming error
			panic(fmt.Errorf("%w: %w", anc.ErrUnmarshal, err))
		}
		sig, err := anc.NewSchnorrEOTSSig(iter.Value())
		if err != nil {
			// failing to unmarshal EOTS sig in KVStore is a programming error
			panic(fmt.Errorf("failed to unmarshal EOTS signature: %w", err))
		}
		sigs[fpBTCPK.MarshalHex()] = sig
	}
	return sigs
}

// GetVoters gets returns a map of voters' BTC PKs to the given height
func (k Keeper) GetVoters(ctx context.Context, height uint64) map[string]struct{} {
	store := k.voteHeightStore(ctx, height)
	iter := store.Iterator(nil, nil)
	defer iter.Close()

	// if there is no vote on this height, return nil
	if !iter.Valid() {
		return nil
	}

	voterBTCPKs := map[string]struct{}{}
	for ; iter.Valid(); iter.Next() {
		// accumulate voterBTCPKs
		fpBTCPK, err := anc.NewBIP340PubKey(iter.Key())
		if err != nil {
			// failing to unmarshal finality provider's BTC PK in KVStore is a programming error
			panic(fmt.Errorf("%w: %w", anc.ErrUnmarshal, err))
		}
		voterBTCPKs[fpBTCPK.MarshalHex()] = struct{}{}
	}
	return voterBTCPKs
}

// voteHeightStore returns the KVStore of the votes
// prefix: VoteKey
// key: (block height || finality provider PK)
// value: EOTS sig
func (k Keeper) voteHeightStore(ctx context.Context, height uint64) prefix.Store {
	prefixedStore := k.voteStore(ctx)
	return prefix.NewStore(prefixedStore, sdk.Uint64ToBigEndian(height))
}

// voteStore returns the KVStore of the votes
// prefix: VoteKey
// key: (prefix)
// value: EOTS sig
func (k Keeper) voteStore(ctx context.Context) prefix.Store {
	storeAdapter := runtime.KVStoreAdapter(k.storeService.OpenKVStore(ctx))
	return prefix.NewStore(storeAdapter, types.VoteKey)
}
