package clientcontroller

import (
	"fmt"
	ancclient "github.com/anon-org/anon/v4/client/client"
	"github.com/anon-org/finality-provider/clientcontroller/api"
	"github.com/anon-org/finality-provider/clientcontroller/anon"
	fpcfg "github.com/anon-org/finality-provider/finality-provider/config"
	"go.uber.org/zap"
)

func NewAnonController(ancConfig *fpcfg.ANCConfig, logger *zap.Logger) (api.AnonController, error) {
	ancCfg := ancConfig.ToAnonConfig()
	ancClient, err := ancclient.New(
		&ancCfg,
		logger,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create Anon rpc client: %w", err)
	}
	cc, err := anon.NewAnonController(ancClient, ancConfig, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to create Anon rpc client: %w", err)
	}

	return cc, nil
}
