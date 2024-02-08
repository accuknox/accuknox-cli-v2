package onboard

var (
	kmuxConfig = `kmux:
  sink:
    stream: {{.StreamName}}

knox-gateway:
  server: {{.ServerURL}}
`

	peaConfig = `server:
  port: :6060
  basepath: /pea

application:
  name: policy-enf-agent

spire:
  enable: true
  agent: "unix:///var/run/spire/agent.sock"

endpoint:
  urlendpoint: /pps/api/v1/policy-provider/fetch-policy
  baseurlendpoint: https://{{.PPSHost}}

statusendpoint:
  endpoint: https://{{.PPSHost}}/pps/api/v1/policy-provider/change-status-policy

syncuptime:
  t: 5

annotation:
  statusendpoint: /pps/api/v1/policy-provider/update-annotation-status
  annotationendpoint: /pps/api/v1/policy-provider/fetch-annotations
  basepath: https://{{.PPSHost}}

non-k8s:
  enable: true
  policy-server-port: 32770
`

	siaConfig = `spire:
  enable: true
  agent: "unix:///var/run/spire/agent.sock"

kmux-topic: shared-event
kmux-topic-prefix: persistent://accuknox/cluster-entity/

heartbeat:
  interval: 5m

kmux-config-file: "/opt/sia/kmux-config.yaml"

state-agent:
  port: 32769

k8s:
  enable:false
`

	spireAgentConfig = `agent {
    data_dir = "/opt/spire-data"
    log_level = "DEBUG"
    trust_domain = "accuknox.com"
    join_token = "{{.JoinToken}}"
    insecure_bootstrap = true

    # spire-server address
    server_address = "{{.SpireHostAddr}}"
    server_port = "{{.SpireHostPort}}"
    #trust_bundle_url = "{{.SpireTrustBundleURL}}"

    # exposing spire-agent
    agent_address = "0.0.0.0"
    agent_port = "9091"
    socket_path ="/var/run/spire/agent.sock"
}

plugins {
    NodeAttestor "join_token" {
        plugin_data {
        }
    }
    KeyManager "disk" {
        plugin_data {
            directory = "/opt/spire-data"
        }
    }
    WorkloadAttestor "docker" {
        plugin_data {
          container_id_cgroup_matchers = []
        }
    }
}

health_checks {
  listener_enabled = true
  bind_address = "0.0.0.0"
  bind_port = "9090"
  live_path = "/live"
  ready_path = "/ready"
}
`
)
