package e2etest

import (
	"github.com/anon-org/vigilante/config"
)

var (
	logger, _ = config.NewRootLogger("auto", "debug")
	log       = logger.Sugar()
)
