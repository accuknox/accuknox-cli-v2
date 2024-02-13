package onboard

import (
	_ "embed"
)

var (
	//go:embed config-files/docker-compose_cp-node.yaml
	cpComposeFileTemplate string

	//go:embed config-files/docker-compose_node.yaml
	workerNodeComposeFileTemplate string
)
