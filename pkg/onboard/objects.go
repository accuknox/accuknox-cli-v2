package onboard

import (
	_ "embed"
)

var (
	//go:embed templates/docker-compose_cp-node.yaml
	cpComposeFileTemplate string

	//go:embed templates/docker-compose_node.yaml
	workerNodeComposeFileTemplate string
)
