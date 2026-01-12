package main

import (
	"rgb-game/config/ledger"
	"rgb-game/pkg/logger"
)

func main() {
	logger.Init()

	config, err := ledger.Init()
	if err != nil {
		return
	}

	logger.Info("Config Inited", "Config", config)
}
