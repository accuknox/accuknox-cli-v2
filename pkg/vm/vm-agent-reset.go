package vm

import (
	"github.com/accuknox/accuknox-cli-v2/pkg/logger"
	"github.com/accuknox/accuknox-cli-v2/pkg/onboard"
)

func Reset(exclude []string) error {

	excludeMap := excludeAgents(exclude)

	logger.Info1("Resetting restart counter for agents")

	return onboard.ResetRestartCount(excludeMap)
}

func excludeAgents(exclude []string) map[string]bool {
	excluded := make(map[string]bool)
	for _, agent := range exclude {
		excluded[agent] = true
	}
	return excluded
}
