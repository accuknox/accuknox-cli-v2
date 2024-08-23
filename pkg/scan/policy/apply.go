package policy

import (
	"context"
	"fmt"
	"os"
	"reflect"
	"sync"

	"github.com/accuknox/accuknox-cli-v2/pkg/common"
	jsoniter "github.com/json-iterator/go"
	katypes "github.com/kubearmor/KubeArmor/KubeArmor/types"
	kaproto "github.com/kubearmor/KubeArmor/protobuf"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"sigs.k8s.io/yaml"
)

const (
	ActionBlock = "Block"
	ActionAudit = "Audit"
)

// Apply policies via gRPC
type Apply struct {
	// grpc connection
	conn *grpc.ClientConn

	// grpc connection string
	connString string

	// policy cache
	policies *GetPolicy

	// policy service client
	policyService kaproto.PolicyServiceClient

	// Name of the host
	hostname string

	// action
	action string

	// event type (add or delete)
	event string

	// dryrun
	dryrun bool

	// generated policies
	generatedPolicies [][]byte
}

// NewApplier will instantiate the policy applier
func NewApplier(connString, zipURL, hostname, action, event string, dryrun bool) *Apply {
	return &Apply{
		connString: connString,
		policies:   NewGenerator(zipURL),
		hostname:   hostname,
		action:     action,
		event:      event,
		dryrun:     dryrun,
	}
}

// Apply connects with gRPC and starts to apply the policies
func (a *Apply) Apply() error {
	err := a.connectToGRPC()
	if err != nil {
		return err
	}

	a.policyService = kaproto.NewPolicyServiceClient(a.conn)

	err = a.policies.FetchTemplates()
	if err != nil {
		return fmt.Errorf("failed to fetch policy templates: %v", err.Error())
	}

	return a.handlePolicies()
}

func (a *Apply) handlePolicies() error {
	var wg sync.WaitGroup
	errorChan := make(chan error, len(a.policies.PolicyCache))

	for _, policy := range a.policies.PolicyCache {
		wg.Add(1)
		go func(p *KubeArmorPolicy) {
			defer wg.Done()
			if err := a.processPolicy(p); err != nil {
				errorChan <- err
			}
		}(policy)
	}

	wg.Wait()
	close(errorChan)

	for err := range errorChan {
		if err != nil {
			return err
		}
	}

	return nil
}

// omitEmpty removes empty fields from a map recursively
func omitEmpty(m map[string]interface{}) map[string]interface{} {
	for k, v := range m {
		if v == nil {
			delete(m, k)
			continue
		}
		switch v := v.(type) {
		case map[string]interface{}:
			m[k] = omitEmpty(v)
			if len(m[k].(map[string]interface{})) == 0 {
				delete(m, k)
			}
		case []interface{}:
			if len(v) == 0 {
				delete(m, k)
			}
		case string:
			if v == "" {
				delete(m, k)
			}
		case int:
			if v == 0 {
				delete(m, k)
			}
		case float64:
			if v == 0 {
				delete(m, k)
			}
		case bool:
			if !v {
				delete(m, k)
			}
		default:
			rv := reflect.ValueOf(v)
			if rv.Kind() == reflect.Struct {
				m[k] = omitEmpty(structToMap(v))
				if len(m[k].(map[string]interface{})) == 0 {
					delete(m, k)
				}
			}
		}
	}
	return m
}

// structToMap converts a struct to a map
func structToMap(obj interface{}) map[string]interface{} {
	out := make(map[string]interface{})
	j, _ := jsoniter.ConfigCompatibleWithStandardLibrary.Marshal(obj)
	jsoniter.ConfigCompatibleWithStandardLibrary.Unmarshal(j, &out)
	return out
}

func (a *Apply) processPolicy(policy *KubeArmorPolicy) error {
	a.modifyPolicy(policy)

	// Convert policy to map and omit empty fields
	policyMap := structToMap(policy)
	cleanedPolicyMap := omitEmpty(policyMap)

	// Marshal the cleaned map back to JSON
	cleanedJSON, err := jsoniter.ConfigCompatibleWithStandardLibrary.Marshal(cleanedPolicyMap)
	if err != nil {
		return fmt.Errorf("failed to marshal cleaned policy: %s", err.Error())
	}

	// Convert JSON to YAML
	policyBytes, err := yaml.JSONToYAML(cleanedJSON)
	if err != nil {
		return fmt.Errorf("failed to convert JSON to YAML: %s", err.Error())
	}

	if a.dryrun {
		a.generatedPolicies = append(a.generatedPolicies, policyBytes)
		return nil
	}

	var hostPolicy katypes.K8sKubeArmorHostPolicy
	err = jsoniter.ConfigCompatibleWithStandardLibrary.Unmarshal(cleanedJSON, &hostPolicy)
	if err != nil {
		return fmt.Errorf("failed to unmarshal to json: %s", err.Error())
	}

	policyEvent := katypes.K8sKubeArmorHostPolicyEvent{
		Type:   a.event,
		Object: hostPolicy,
	}

	policyEventBytes, err := jsoniter.ConfigCompatibleWithStandardLibrary.Marshal(policyEvent)
	if err != nil {
		return fmt.Errorf("failed to marshal the policy event: %s", err.Error())
	}

	return a.applyPolicy(policyEventBytes)
}

func (a *Apply) modifyPolicy(policy *KubeArmorPolicy) {
	if policy.Spec.NodeSelector.MatchLabels == nil {
		policy.Spec.NodeSelector.MatchLabels = make(map[string]string)
	}

	// Always set the hostname to a.hostname, regardless of what's in the policy
	policy.Spec.NodeSelector.MatchLabels["kubearmor.io/hostname"] = a.hostname

	// Remove kubernetes.io/hostname if it exists
	delete(policy.Spec.NodeSelector.MatchLabels, "kubernetes.io/hostname")

	newAction := a.action

	if policy.Spec.Action != "" {
		policy.Spec.Action = newAction
	}

	// Update Process actions
	updateProcessActions(&policy.Spec.Process, newAction)

	// Update File actions
	updateFileActions(&policy.Spec.File, newAction)

	// Update Network actions
	updateNetworkActions(&policy.Spec.Network, newAction)

	// Update Capabilities actions
	updateCapabilitiesActions(&policy.Spec.Capabilities, newAction)
}

func updateProcessActions(process *ProcessType, newAction string) {
	if process.Action != "" {
		process.Action = newAction
	}
	for i := range process.MatchPaths {
		if process.MatchPaths[i].Action != "" {
			process.MatchPaths[i].Action = newAction
		}
	}
	for i := range process.MatchDirectories {
		if process.MatchDirectories[i].Action != "" {
			process.MatchDirectories[i].Action = newAction
		}
	}
	for i := range process.MatchPatterns {
		if process.MatchPatterns[i].Action != "" {
			process.MatchPatterns[i].Action = newAction
		}
	}
}

func updateFileActions(file *FileType, newAction string) {
	if file.Action != "" {
		file.Action = newAction
	}
	for i := range file.MatchPaths {
		if file.MatchPaths[i].Action != "" {
			file.MatchPaths[i].Action = newAction
		}
	}
	for i := range file.MatchDirectories {
		if file.MatchDirectories[i].Action != "" {
			file.MatchDirectories[i].Action = newAction
		}
	}
	for i := range file.MatchPatterns {
		if file.MatchPatterns[i].Action != "" {
			file.MatchPatterns[i].Action = newAction
		}
	}
}

func updateNetworkActions(network *NetworkType, newAction string) {
	if network.Action != "" {
		network.Action = newAction
	}
	for i := range network.MatchProtocols {
		if network.MatchProtocols[i].Action != "" {
			network.MatchProtocols[i].Action = newAction
		}
	}
}

func updateCapabilitiesActions(capabilities *CapabilitiesType, newAction string) {
	if capabilities.Action != "" {
		capabilities.Action = newAction
	}
	for i := range capabilities.MatchCapabilities {
		if capabilities.MatchCapabilities[i].Action != "" {
			capabilities.MatchCapabilities[i].Action = newAction
		}
	}
}

func (a *Apply) applyPolicy(policyBytes []byte) error {
	req := kaproto.Policy{
		Policy: policyBytes,
	}

	resp, err := a.policyService.HostPolicy(context.Background(), &req)
	if err != nil {
		return fmt.Errorf("failed to send policy over grpc: %s", err.Error())
	}

	fmt.Printf("Policy %s \n", resp.Status)
	return nil
}

func (a *Apply) connectToGRPC() error {
	opts := []grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(32 * 10e6)),
	}

	if a.connString == "" {
		a.connString = common.KubeArmorGRPCAddress
	}
	connection, err := grpc.Dial(a.connString, opts...)
	if err != nil {
		return err
	}

	a.conn = connection
	return nil
}

// SavePolicies saves the generated policies to a file in YAML format
func (a *Apply) SavePolicies(filePath string) error {
	if !a.dryrun || len(a.generatedPolicies) == 0 {
		return fmt.Errorf("no policies to save or not in dry run mode")
	}

	file, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("failed to create file: %s", err.Error())
	}
	defer file.Close()

	for i, policy := range a.generatedPolicies {
		if i > 0 {
			_, err = file.WriteString("---\n")
			if err != nil {
				return fmt.Errorf("failed to write separator: %s", err.Error())
			}
		}

		_, err = file.Write(policy)
		if err != nil {
			return fmt.Errorf("failed to write policy: %s", err.Error())
		}

		_, err = file.WriteString("\n")
		if err != nil {
			return fmt.Errorf("failed to write newline: %s", err.Error())
		}
	}

	return nil
}
