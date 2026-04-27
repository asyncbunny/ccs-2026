package client

import (
	"time"

	"github.com/anon-org/anon/v4/client/anonclient"

	anc "github.com/anon-org/anon/v4/app"
	"github.com/anon-org/anon/v4/client/config"
	"github.com/anon-org/anon/v4/client/query"
	rpchttp "github.com/cometbft/cometbft/rpc/client/http"
	"go.uber.org/zap"
)

type Client struct {
	*query.QueryClient

	provider *anonclient.CosmosProvider
	timeout  time.Duration
	logger   *zap.Logger
	cfg      *config.AnonConfig
}

func (c *Client) Provider() *anonclient.CosmosProvider {
	return c.provider
}

func New(cfg *config.AnonConfig, logger *zap.Logger) (*Client, error) {
	var (
		zapLogger *zap.Logger
		err       error
	)

	// ensure cfg is valid
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	// use the existing logger or create a new one if not given
	zapLogger = logger
	if zapLogger == nil {
		zapLogger, err = newRootLogger("console", true)
		if err != nil {
			return nil, err
		}
	}

	provider, err := cfg.ToCosmosProviderConfig().NewProvider(
		"", // TODO: set home path
		"anon",
	)
	if err != nil {
		return nil, err
	}

	cp := provider.(*anonclient.CosmosProvider)
	cp.PCfg.KeyDirectory = cfg.KeyDirectory

	// Create tmp Anon app to retrieve and register codecs
	// Need to override this manually as otherwise option from config is ignored
	cp.Cdc = anc.GetEncodingConfig()

	// initialise Cosmos provider
	// NOTE: this will create a RPC client. The RPC client will be used for
	// submitting txs and making ad hoc queries. It won't create WebSocket
	// connection with Anon node
	if err = cp.Init(); err != nil {
		return nil, err
	}

	// create a queryClient so that the Client inherits all query functions
	// TODO: merge this RPC client with the one in `cp` after Cosmos side
	// finishes the migration to new RPC client
	// see https://github.com/strangelove-ventures/cometbft-client
	c, err := rpchttp.NewWithTimeout(cp.PCfg.RPCAddr, "/websocket", uint(cfg.Timeout.Seconds()))
	if err != nil {
		return nil, err
	}
	queryClient, err := query.NewWithClient(c, cfg.Timeout)
	if err != nil {
		return nil, err
	}

	return &Client{
		queryClient,
		cp,
		cfg.Timeout,
		zapLogger,
		cfg,
	}, nil
}

func (c *Client) GetConfig() *config.AnonConfig {
	return c.cfg
}
