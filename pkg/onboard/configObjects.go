package onboard

import (
	_ "embed"
)

var (
	//go:embed templates/kmux-config.yaml
	kmuxConfig string

	//go:embed templates/pea-config.yaml
	peaConfig string

	//go:embed templates/sia-config.yaml
	siaConfig string

	//go:embed templates/spire-agent.conf
	spireAgentConfig string
)
