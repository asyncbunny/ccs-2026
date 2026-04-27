package reporter

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/avast/retry-go/v4"
	anctypes "github.com/anon-org/anon/v4/types"
	"github.com/anon-org/vigilante/types"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
)

var (
	bootstrapAttempts      = uint(60)
	bootstrapAttemptsAtt   = retry.Attempts(bootstrapAttempts)
	bootstrapRetryInterval = retry.Delay(30 * time.Second)
	bootstrapDelayType     = retry.DelayType(retry.FixedDelay)
	bootstrapErrReportType = retry.LastErrorOnly(true)
)

type consistencyCheckInfo struct {
	ancLatestBlockHeight uint32
	startSyncHeight      uint32
}

// checkConsistency checks whether the `max(anc_tip_height - confirmation_depth, anc_base_height)` block is same
// between ANC header chain and BTC main chain.` This makes sure that already confirmed chain is the same from point
// of view of both chains.
func (r *Reporter) checkConsistency() (*consistencyCheckInfo, error) {
	tipRes, err := r.anonClient.BTCHeaderChainTip()
	if err != nil {
		return nil, err
	}

	// Find the base height of ANC header chain
	baseRes, err := r.anonClient.BTCBaseHeader()
	if err != nil {
		return nil, err
	}

	var consistencyCheckHeight uint32
	if tipRes.Header.Height >= baseRes.Header.Height+r.btcConfirmationDepth {
		consistencyCheckHeight = tipRes.Header.Height - r.btcConfirmationDepth
	} else {
		consistencyCheckHeight = baseRes.Header.Height
	}

	// this checks whether header at already confirmed height is the same in reporter btc cache and in anon btc light client
	if err := r.checkHeaderConsistency(consistencyCheckHeight); err != nil {
		return nil, err
	}

	return &consistencyCheckInfo{
		ancLatestBlockHeight: tipRes.Header.Height,
		// we are staring from the block after already confirmed block
		startSyncHeight: consistencyCheckHeight + 1,
	}, nil
}

func (r *Reporter) bootstrap() error {
	var (
		btcLatestBlockHeight uint64
		ibs                  []*types.IndexedBlock
		err                  error
	)

	r.bootstrapMutex.Lock()
	if r.bootstrapInProgress {
		r.bootstrapMutex.Unlock()
		// we only allow one bootstrap process at a time
		return fmt.Errorf("bootstrap already in progress")
	}
	r.bootstrapInProgress = true
	r.bootstrapWg.Add(1)
	r.bootstrapMutex.Unlock()

	// flag to indicate if we should clean up, err happened
	success := false
	defer func() {
		// cleanup func in case we error, prevents deadlocks
		if !success {
			r.bootstrapMutex.Lock()
			r.bootstrapInProgress = false
			r.bootstrapMutex.Unlock()
			r.bootstrapWg.Done()
		}
	}()

	// ensure BTC has caught up with ANC header chain
	if err := r.waitUntilBTCSync(); err != nil {
		return err
	}

	// initialize cache with the latest blocks
	if err := r.initBTCCache(); err != nil {
		return err
	}
	r.logger.Debugf("BTC cache size: %d", r.btcCache.Size())

	consistencyInfo, err := r.checkConsistency()
	if err != nil {
		return err
	}

	ibs, err = r.btcCache.GetLastBlocks(consistencyInfo.startSyncHeight)
	if err != nil {
		panic(err)
	}

	signer := r.anonClient.MustGetAddr()

	r.logger.Infof("BTC height: %d. BTCLightclient height: %d. Start syncing from height %d.",
		btcLatestBlockHeight, consistencyInfo.ancLatestBlockHeight, consistencyInfo.startSyncHeight)

	// extracts and submits headers for each block in ibs
	// Note: As we are retrieving blocks from btc cache from block just after confirmed block which
	// we already checked for consistency, we can be sure that even if rest of the block headers is different than in Anon
	// due to reorg, our fork will be better than the one in Anon.
	if _, err = r.ProcessHeaders(signer, ibs); err != nil {
		// this can happen when there are two contentious vigilantes or if our btc node is behind.
		r.logger.Errorf("Failed to submit headers: %v", err)
		// returning error as it is up to the caller to decide what do next
		return err
	}

	// trim cache to the latest k+w blocks on BTC (which are same as in ANC)
	maxEntries := r.btcConfirmationDepth + r.checkpointFinalizationTimeout
	if err = r.btcCache.Resize(maxEntries); err != nil {
		r.logger.Errorf("Failed to resize BTC cache: %v", err)
		panic(err)
	}
	r.btcCache.Trim()

	r.logger.Infof("Size of the BTC cache: %d", r.btcCache.Size())

	// fetch k+w blocks from cache and submit checkpoints
	ibs = r.btcCache.GetAllBlocks()
	go func() {
		defer func() {
			r.bootstrapMutex.Lock()
			r.bootstrapInProgress = false
			r.bootstrapMutex.Unlock()
			r.bootstrapWg.Done()
		}()
		r.logger.Infof("Async processing checkpoints started")
		_, _ = r.ProcessCheckpoints(signer, ibs)
	}()

	r.logger.Info("Successfully finished bootstrapping")

	success = true

	return nil
}

func (r *Reporter) reporterQuitCtx() (context.Context, func()) {
	quit := r.quitChan()
	ctx, cancel := context.WithCancel(context.Background())
	r.wg.Add(1)
	go func() {
		defer cancel()
		defer r.wg.Done()

		select {
		case <-quit:

		case <-ctx.Done():
		}
	}()

	return ctx, cancel
}

func (r *Reporter) bootstrapWithRetries() {
	// if we are exiting, we need to cancel this process
	ctx, cancel := r.reporterQuitCtx()
	defer cancel()
	if err := retry.Do(func() error {
		// we don't want to allow concurrent bootstrap process, if bootstrap is already in progress
		// we should wait for it to finish
		r.bootstrapWg.Wait()

		return r.bootstrap()
	},
		retry.Context(ctx),
		bootstrapAttemptsAtt,
		bootstrapRetryInterval,
		bootstrapDelayType,
		bootstrapErrReportType, retry.OnRetry(func(n uint, err error) {
			r.logger.Warnf("Failed to bootstap reporter: %v. Attempt: %d, Max attempts: %d", err, n+1, bootstrapAttempts)
		})); err != nil {
		if errors.Is(err, context.Canceled) {
			// context was cancelled we do not need to anything more, app is quiting
			return
		}

		// we failed to bootstrap multiple time, we should panic as something unexpected is happening.
		r.logger.Fatalf("Failed to bootstrap reporter: %v after %d attempts", err, bootstrapAttempts)
	}
}

// initBTCCache fetches the blocks since T-k-w in the BTC canonical chain
// where T is the height of the latest block in ANC header chain
func (r *Reporter) initBTCCache() error {
	var (
		err                  error
		ancLatestBlockHeight uint32
		ancBaseHeight        uint32
		baseHeight           uint32
		ibs                  []*types.IndexedBlock
	)

	r.btcCache, err = types.NewBTCCache(r.cfg.BTCCacheSize) // TODO: give an option to be unsized
	if err != nil {
		panic(err)
	}

	// get T, i.e., total block count in ANC header chain
	tipRes, err := r.anonClient.BTCHeaderChainTip()
	if err != nil {
		return err
	}
	ancLatestBlockHeight = tipRes.Header.Height

	// Find the base height
	baseRes, err := r.anonClient.BTCBaseHeader()
	if err != nil {
		return err
	}
	ancBaseHeight = baseRes.Header.Height

	// Fetch block since `baseHeight = T - k - w` from BTC, where
	// - T is total block count in ANC header chain
	// - k is btcConfirmationDepth of ANC
	// - w is checkpointFinalizationTimeout of ANC
	if ancLatestBlockHeight > ancBaseHeight+r.btcConfirmationDepth+r.checkpointFinalizationTimeout {
		baseHeight = ancLatestBlockHeight - r.btcConfirmationDepth - r.checkpointFinalizationTimeout + 1
	} else {
		baseHeight = ancBaseHeight
	}

	ibs, err = r.btcClient.FindTailBlocksByHeight(baseHeight)
	if err != nil {
		panic(err)
	}

	if err = r.btcCache.Init(ibs); err != nil {
		panic(err)
	}

	return nil
}

// waitUntilBTCSync waits for BTC to synchronize until BTC is no shorter than Anon's BTC light client.
// It returns BTC last block hash, BTC last block height, and Anon's base height.
func (r *Reporter) waitUntilBTCSync() error {
	var (
		btcLatestBlockHeight uint32
		ancLatestBlockHash   *chainhash.Hash
		ancLatestBlockHeight uint32
		err                  error
	)

	// Retrieve hash/height of the latest block in BTC
	btcLatestBlockHeight, err = r.btcClient.GetBestBlock()
	if err != nil {
		return err
	}
	r.logger.Infof("BTC latest block hash and height: (%d)", btcLatestBlockHeight)

	// TODO: if BTC falls behind BTCLightclient's base header, then the vigilante is incorrectly configured and should panic

	// Retrieve hash/height of the latest block in ANC header chain
	tipRes, err := r.anonClient.BTCHeaderChainTip()
	if err != nil {
		return err
	}

	hash, err := anctypes.NewBTCHeaderHashBytesFromHex(tipRes.Header.HashHex)
	if err != nil {
		return err
	}

	ancLatestBlockHash = hash.ToChainhash()
	ancLatestBlockHeight = tipRes.Header.Height
	r.logger.Infof("ANC header chain latest block hash and height: (%v, %d)", ancLatestBlockHash, ancLatestBlockHeight)

	// If BTC chain is shorter than ANC header chain, pause until BTC catches up
	if btcLatestBlockHeight == 0 || btcLatestBlockHeight < ancLatestBlockHeight {
		r.logger.Infof("BTC chain (length %d) falls behind ANC header chain (length %d), wait until BTC catches up", btcLatestBlockHeight, ancLatestBlockHeight)

		// periodically check if BTC catches up with ANC.
		// When BTC catches up, break and continue the bootstrapping process
		ticker := time.NewTicker(5 * time.Second) // TODO: parameterise the polling interval
		for range ticker.C {
			btcLatestBlockHeight, err = r.btcClient.GetBestBlock()
			if err != nil {
				return err
			}
			tipRes, err = r.anonClient.BTCHeaderChainTip()
			if err != nil {
				return err
			}
			ancLatestBlockHeight = tipRes.Header.Height
			if btcLatestBlockHeight > 0 && btcLatestBlockHeight >= ancLatestBlockHeight {
				r.logger.Infof("BTC chain (length %d) now catches up with ANC header chain (length %d), continue bootstrapping", btcLatestBlockHeight, ancLatestBlockHeight)

				break
			}
			r.logger.Infof("BTC chain (length %d) still falls behind ANC header chain (length %d), keep waiting", btcLatestBlockHeight, ancLatestBlockHeight)
		}
	}

	return nil
}

func (r *Reporter) checkHeaderConsistency(consistencyCheckHeight uint32) error {
	var err error

	consistencyCheckBlock := r.btcCache.FindBlock(consistencyCheckHeight)
	if consistencyCheckBlock == nil {
		err = fmt.Errorf("cannot find the %d-th block of ANC header chain in BTC cache for initial consistency check", consistencyCheckHeight)
		panic(err)
	}
	consistencyCheckHash := consistencyCheckBlock.BlockHash()

	r.logger.Debugf("block for consistency check: height %d, hash %v", consistencyCheckHeight, consistencyCheckHash)

	// Given that two consecutive BTC headers are chained via hash functions,
	// generating a header that can be in two different positions in two different BTC header chains
	// is as hard as breaking the hash function.
	// So as long as the block exists on Anon, it has to be at the same position as in Anon as well.
	res, err := r.anonClient.ContainsBTCBlock(&consistencyCheckHash) // TODO: this API has error. Find out why
	if err != nil {
		return err
	}
	if !res.Contains {
		err = fmt.Errorf("BTC main chain is inconsistent with ANC header chain: k-deep block in ANC header chain: %v", consistencyCheckHash)
		panic(err)
	}

	return nil
}
