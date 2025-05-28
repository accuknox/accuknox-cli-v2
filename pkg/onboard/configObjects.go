package onboard

import (
	_ "embed"
)

var (
	//go:embed templates/kmux-config.yaml
	kmuxConfig string

	//go:embed templates/publisher-kmux-config.yaml
	kmuxPublisherConfig string

	//go:embed templates/consumer-kmux-config.yaml
	kmuxConsumerConfig string

	//go:embed templates/pea-config.yaml
	peaConfig string

	//go:embed templates/sia-config.yaml
	siaConfig string

	//go:embed templates/discover-config.yaml
	discoverConfig string

	//go:embed templates/sumengine-config.yaml
	sumEngineConfig string

	//go:embed templates/hardening-agent-config.yaml
	hardeningAgentConfig string

	//go:embed templates/systemdTemplates/kubearmor-config.yaml
	kubeArmorConfig string

	//go:embed templates/systemdTemplates/vm-adapter-config.yaml
	vmAdapterConfig string

	//go:embed templates/spire-agent.conf
	spireAgentConfig string

	//go:embed templates/rabbitmq.conf
	rabbitmqConfig string

	//go:embed templates/definitions.json
	rabbitmqDefinitions string

	//go:embed templates/systemdTemplates/feeder-service-env
	fsEnvVal string

	spireTrustBundleURLMap = map[string]string{
		"dev":     "https://accuknox-dev-cert-spire.s3.us-east-2.amazonaws.com/ca.crt",
		"stage":   "https://accuknox-stage-cert-spire.s3.us-east-2.amazonaws.com/ca.crt",
		"demo":    "https://accuknox-demo-cert-spire.s3.us-east-2.amazonaws.com/ca.crt",
		"prod":    "https://accuknox-prod-cert-spire.s3.us-east-2.amazonaws.com/ca.crt",
		"xcitium": "https://accuknox-spire.s3.amazonaws.com/certs/xcitium/certificate.crt",
	}
)

const (
	SpireDev     = "spire.dev.accuknox.com"
	SpireStage   = "spire.stage.accuknox.com"
	SpireDemo    = "spire.demo.accuknox.com"
	SpireProd    = "spire.accuknox.com"
	SpireXcitium = "spire.xcitium.accuknox.com"
)
