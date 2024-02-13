package onboard

import (
	_ "embed"
)

var (
	//go:embed config-files/kmux-config.yaml
	kmuxConfig string

	//go:embed config-files/pea-config.yaml
	peaConfig string

	//go:embed config-files/sia-config.yaml
	siaConfig string

	//go:embed config-files/spire-agent.conf
	spireAgentConfig string
)
