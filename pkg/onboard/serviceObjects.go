package onboard

import (
	_ "embed"
)

var (
	//go:embed templates/systemdTemplates/kubearmor.service
	kubearmorServiceFile string

	//go:embed templates/systemdTemplates/feeder-service.service
	feederServiceFile string

	//go:embed templates/systemdTemplates/pea.service
	peaServiceFile string

	//go:embed templates/systemdTemplates/relay-server.service
	relayServerServiceFile string

	//go:embed templates/systemdTemplates/sia.service
	siaServiceFile string

	//go:embed templates/systemdTemplates/vm-adapter.service
	vmAdapterServiceFile string

	//go:embed templates/systemdTemplates/sumengine.service
	sumEngineFile string

	//go:embed templates/systemdTemplates/discover.service
	discoverFile string

	//go:embed templates/systemdTemplates/hardening-agent.service
	hardeningAgentFile string

	//go:embed templates/systemdTemplates/rra.service
	rraServiceFile string

	//go:embed templates/systemdTemplates/rra.timer
	rraTimerFile string

	//go:embed templates/systemdTemplates/logrotate
	logRotateFile string

	//go:embed templates/systemdTemplates/spire-agent.service
	spireAgentFile string
)
