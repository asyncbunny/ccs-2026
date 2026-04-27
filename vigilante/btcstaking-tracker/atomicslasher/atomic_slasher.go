package atomicslasher

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/anon-org/vigilante/btcclient"
	"github.com/anon-org/vigilante/config"
	"github.com/anon-org/vigilante/metrics"
	"github.com/btcsuite/btcd/btcec/v2"
	notifier "github.com/lightningnetwork/lnd/chainntnfs"
	"go.uber.org/zap"
)

type AtomicSlasher struct {
	// service-related fields
	startOnce sync.Once
	stopOnce  sync.Once
	wg        sync.WaitGroup
	quit      chan struct{}

	// internal components
	logger      *zap.Logger
	btcClient   btcclient.BTCClient
	btcNotifier notifier.ChainNotifier
	ancAdapter  *AnonAdapter

	// config parameters
	cfg               *config.BTCStakingTrackerConfig
	retrySleepTime    time.Duration
	maxRetrySleepTime time.Duration

	// system states
	btcTipHeight    atomic.Uint32
	btcDelIndex     *BTCDelegationIndex
	slashingTxChan  chan *SlashingTxInfo
	slashedFPSKChan chan<- *btcec.PrivateKey // channel for SKs of slashed finality providers

	// metrics
	metrics *metrics.AtomicSlasherMetrics
}

func New(
	cfg *config.BTCStakingTrackerConfig,
	parentLogger *zap.Logger,
	retrySleepTime time.Duration,
	maxRetrySleepTime time.Duration,
	maxRetryTimes uint,
	btcClient btcclient.BTCClient,
	btcNotifier notifier.ChainNotifier,
	ancClient AnonClient,
	slashedFPSKChan chan *btcec.PrivateKey,
	metrics *metrics.AtomicSlasherMetrics,
) *AtomicSlasher {
	logger := parentLogger.With(zap.String("module", "atomic_slasher"))
	ancAdapter := NewAnonAdapter(logger, cfg, retrySleepTime, maxRetrySleepTime, maxRetryTimes, ancClient)

	return &AtomicSlasher{
		quit:              make(chan struct{}),
		cfg:               cfg,
		retrySleepTime:    retrySleepTime,
		maxRetrySleepTime: maxRetrySleepTime,
		logger:            logger,
		btcClient:         btcClient,
		btcNotifier:       btcNotifier,
		ancAdapter:        ancAdapter,
		btcDelIndex:       NewBTCDelegationIndex(),
		slashingTxChan:    make(chan *SlashingTxInfo, 100), // TODO: parameterise
		slashedFPSKChan:   slashedFPSKChan,
		metrics:           metrics,
	}

	// TODO: initialisation that slashes all culpable finality providers since
	// the BTC height that the atomic slasher shuts down
}

func (as *AtomicSlasher) Start() error {
	var startErr error
	as.startOnce.Do(func() {
		as.logger.Info("starting atomic slasher")

		as.wg.Add(3)
		go as.slashingTxTracker()
		go as.btcDelegationTracker()
		go as.selectiveSlashingReporter()

		as.logger.Info("atomic slasher started")
	})

	return startErr
}

func (as *AtomicSlasher) Stop() error {
	var stopErr error
	as.stopOnce.Do(func() {
		as.logger.Info("stopping atomic slasher")
		close(as.quit)
		close(as.slashingTxChan)
		as.wg.Wait()
		as.logger.Info("stopping atomic slasher")
	})

	return stopErr
}

func (as *AtomicSlasher) quitContext() (context.Context, func()) {
	ctx, cancel := context.WithCancel(context.Background())
	as.wg.Add(1)
	go func() {
		defer cancel()
		defer as.wg.Done()

		select {
		case <-as.quit:

		case <-ctx.Done():
		}
	}()

	return ctx, cancel
}
