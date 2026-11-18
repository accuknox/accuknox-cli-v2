package onboard

import (
	"fmt"
	"golang.org/x/mod/semver"
)

func (cc *ClusterConfig) summarySchema() string {
	if cc.SummarySchemaVersion == "" {
		return "v2"
	}
	return cc.SummarySchemaVersion
}

// ValidateSummarySchema rejects configurations whose local consumer cannot decode batches.
func (cc *ClusterConfig) ValidateSummarySchema() error {
	schema := cc.summarySchema()
	if schema != "v2" && schema != "v3" {
		return fmt.Errorf("summary-schema-version must be v2 or v3, got %q", schema)
	}
	if schema == "v3" && (cc.DeployDiscover || semver.Compare(cc.AgentsVersion, "v0.11.0") < 0) {
		return fmt.Errorf("summary v3 requires local discovery to be disabled and agents version >= v0.11.0; the local discover consumer only supports v2")
	}
	return nil
}

// Apply the configured topic prefix consistently to both summary schemas.
func (cc *ClusterConfig) summaryTopic() string {
	if cc.summarySchema() == "v3" {
		return getTopicName(cc.RMQTopicPrefix, "summary-v3")
	}
	return getTopicName(cc.RMQTopicPrefix, "summary-v2")
}
