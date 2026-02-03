package onboard

import "testing"

func TestClusterConfig_placeServiceFiles(t *testing.T) {
	tests := []struct {
		name string // description of this test case
		// Named input parameters for receiver constructor.
		clusterType                ClusterType
		userConfigPath             string
		vmMode                     VMMode
		vmAdapterTag               string
		kubeArmorRelayServerTag    string
		peaVersionTag              string
		siaVersionTag              string
		feederVersionTag           string
		sumEngineTag               string
		discoverVersionTag         string
		hardeningAgentVersionTag   string
		kubearmorVersion           string
		releaseVersion             string
		kubearmorImage             string
		kubearmorInitImage         string
		vmAdapterImage             string
		relayServerImage           string
		siaImage                   string
		peaImage                   string
		feederImage                string
		rmqImage                   string
		sumEngineImage             string
		hardeningAgentImage        string
		spireImage                 string
		waitForItImage             string
		discoverImage              string
		nodeAddress                string
		dryRun                     bool
		workerNode                 bool
		deployRMQ                  bool
		imagePullPolicy            string
		visibility                 string
		hostVisibility             string
		sumengineViz               string
		audit                      string
		block                      string
		hostAudit                  string
		hostBlock                  string
		alertThrottling            bool
		maxAlertPerSec             int
		throttleSec                int
		cidr                       string
		secureContainers           bool
		skipBTF                    bool
		systemMonitorPath          string
		rmqAddr                    string
		deploySumengine            bool
		registry                   string
		registryConfigPath         string
		insecureRegistryConnection bool
		httpRegistryConnection     bool
		preserveUpstream           bool
		topicPrefix                string
		connName                   string
		sumEngineCronTime          string
		tls                        TLS
		enableHostPolicyDiscovery  bool
		splunk                     SplunkConfig
		stateRefreshTime           int
		spireEnabled               bool
		spireCert                  bool
		logRotate                  string
		parallel                   int
		hardeningService           bool
		releaseFile                string
		proxy                      Proxy
		deployDiscover             bool
		wantErr                    bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cc, err := CreateClusterConfig(tt.clusterType, tt.userConfigPath, tt.vmMode, tt.vmAdapterTag, tt.kubeArmorRelayServerTag, tt.peaVersionTag, tt.siaVersionTag, tt.feederVersionTag, tt.sumEngineTag, tt.discoverVersionTag, tt.hardeningAgentVersionTag, tt.kubearmorVersion, tt.releaseVersion, tt.kubearmorImage, tt.kubearmorInitImage, tt.vmAdapterImage, tt.relayServerImage, tt.siaImage, tt.peaImage, tt.feederImage, tt.rmqImage, tt.sumEngineImage, tt.hardeningAgentImage, tt.spireImage, tt.waitForItImage, tt.discoverImage, tt.nodeAddress, tt.dryRun, tt.workerNode, tt.deployRMQ, tt.imagePullPolicy, tt.visibility, tt.hostVisibility, tt.sumengineViz, tt.audit, tt.block, tt.hostAudit, tt.hostBlock, tt.alertThrottling, tt.maxAlertPerSec, tt.throttleSec, tt.cidr, tt.secureContainers, tt.skipBTF, tt.systemMonitorPath, tt.rmqAddr, tt.deploySumengine, tt.registry, tt.registryConfigPath, tt.insecureRegistryConnection, tt.httpRegistryConnection, tt.preserveUpstream, tt.topicPrefix, tt.connName, tt.sumEngineCronTime, tt.tls, tt.enableHostPolicyDiscovery, tt.splunk, tt.stateRefreshTime, tt.spireEnabled, tt.spireCert, tt.logRotate, tt.parallel, tt.hardeningService, tt.releaseFile, tt.proxy, tt.deployDiscover)
			if err != nil {
				t.Fatalf("could not construct receiver type: %v", err)
			}
			gotErr := cc.placeServiceFiles()
			if gotErr != nil {
				if !tt.wantErr {
					t.Errorf("placeServiceFiles() failed: %v", gotErr)
				}
				return
			}
			if tt.wantErr {
				t.Fatal("placeServiceFiles() succeeded unexpectedly")
			}
		})
	}
}
