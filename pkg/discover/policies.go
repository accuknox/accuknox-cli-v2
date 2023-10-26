package discover

import (
	"context"
	"errors"
	"fmt"
	"github.com/accuknox/accuknox-cli-v2/pkg"
	"strconv"
	"strings"

	"github.com/clarketm/json"
	"github.com/kubearmor/kubearmor-client/k8s"
	"github.com/kubearmor/kubearmor-client/utils"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"sigs.k8s.io/yaml"

	dev2policy "github.com/accuknox/dev2/api/grpc/v1/policy"
	policyType "github.com/accuknox/dev2/discover/pkg/common"
	log "github.com/sirupsen/logrus"
)

var matchLabels = map[string]string{"app": pkg.ServiceName}
var connection *grpc.ClientConn

func initClientConnection(c *k8s.Client, o Options) error {
	if connection != nil {
		return nil
	}
	var err error
	connection, err = getClientConnection(c, o)
	if err != nil {
		return err
	}
	log.Info("Connected to discovery engine")
	return nil
}

func getClientConnection(c *k8s.Client, o Options) (*grpc.ClientConn, error) {
	gRPC := ""
	targetSvc := pkg.ServiceName

	if o.GRPC != "" {
		gRPC = o.GRPC
	} else {
		pf, err := utils.InitiatePortForward(c, pkg.Port, pkg.Port, matchLabels, targetSvc)
		if err != nil {
			return nil, err
		}
		gRPC = "localhost:" + strconv.FormatInt(pf.LocalPort, 10)
		log.Info(gRPC)
	}

	conn, err := grpc.Dial(gRPC, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, err
	}

	return conn, nil
}

func disconnect() {
	if connection != nil {
		err := connection.Close()
		if err != nil {
			log.WithError(err).Error("failed to close connection")
		} else {
			log.Info("Disconnected successfully from discovery engine")
		}
	}
}

func getNetworkPolicy(c *k8s.Client, o Options) ([]string, error) {
	var data []string

	err := initClientConnection(c, o)
	if err != nil {
		log.WithError(err).Error("failed to connect to discovery engine")
		return nil, err
	}

	client := dev2policy.NewGetPolicyClient(connection)
	resp, err := client.GetPolicy(context.Background(), &dev2policy.PolicyRequest{
		Type: PolicyType, // discovered
		Kind: o.Kind,     // NetworkPolicy
	})
	if err != nil {
		log.WithError(err).Error("failed to fetch response from discovery engine")
		return nil, err
	}

	if resp != nil {
		for _, policy := range resp.Policies {
			policyString := string(policy.Yaml)

			var networkPolicy policyType.KnoxNetworkPolicy
			err := yaml.Unmarshal([]byte(policyString), &networkPolicy)
			if err != nil {
				log.WithError(err).Error("failed to unmarshal " + KindK8sNetworkPolicy)
				continue
			}

			if !networkPolicyFilter(networkPolicy, o) {
				continue
			}

			formattedPolicy, err := formatPolicy(networkPolicy, o)
			if err != nil {
				log.WithError(err).Error("failed to format " + KindK8sNetworkPolicy)
				return nil, err
			}

			data = append(data, formattedPolicy)
		}
		return data, err
	}

	return data, err
}

func getKaHostPolicy(c *k8s.Client, o Options) ([]string, error) {
	var data []string

	err := initClientConnection(c, o)
	if err != nil {
		log.WithError(err).Error("failed to connect to discovery engine")
		return nil, err
	}

	client := dev2policy.NewGetPolicyClient(connection)
	resp, err := client.GetPolicy(context.Background(), &dev2policy.PolicyRequest{
		Type: PolicyType, // discovered
		Kind: o.Kind,     // KAHostPolicy
	})
	if err != nil {
		log.WithError(err).Error("failed to fetch response from discovery engine")
		return nil, err
	}

	if resp != nil {
		for _, policy := range resp.Policies {
			policyString := string(policy.Yaml)

			var kaHostPolicy policyType.KubeArmorPolicy
			err := yaml.Unmarshal([]byte(policyString), &kaHostPolicy)
			if err != nil {
				log.WithError(err).Error("failed to unmarshal " + KindKubeArmorHostPolicy)
				continue
			}

			if !kaPolicyFilter(kaHostPolicy, o) {
				continue
			}

			formattedPolicy, err := formatPolicy(kaHostPolicy, o)
			if err != nil {
				log.WithError(err).Error("failed to format " + KindKubeArmorHostPolicy)
				continue
			}

			data = append(data, formattedPolicy)
		}
		return data, err
	}

	return data, nil
}

func getKaPolicy(c *k8s.Client, o Options) ([]string, error) {
	var data []string

	err := initClientConnection(c, o)
	if err != nil {
		log.WithError(err).Error("failed to connect to discovery engine")
		return nil, err
	}

	client := dev2policy.NewGetPolicyClient(connection)
	resp, err := client.GetPolicy(context.Background(), &dev2policy.PolicyRequest{
		Type: PolicyType, // discovered
		Kind: o.Kind,     // KAPolicy
	})
	if err != nil {
		log.WithError(err).Error("failed to fetch response from discovery engine")
		return nil, err
	}

	if resp != nil {
		for _, policy := range resp.Policies {
			policyString := string(policy.Yaml)

			var kaPolicy policyType.KubeArmorPolicy
			err := yaml.Unmarshal([]byte(policyString), &kaPolicy)
			if err != nil {
				log.WithError(err).Error("failed to unmarshal " + KindKubeArmorPolicy)
				continue
			}

			if !kaPolicyFilter(kaPolicy, o) {
				continue
			}

			formattedPolicy, err := formatPolicy(kaPolicy, o)
			if err != nil {
				log.WithError(err).Error("failed to format " + KindKubeArmorPolicy)
				return nil, err
			}

			data = append(data, formattedPolicy)
		}
		return data, err
	}

	return data, nil
}

// Centralized filteration based on user options,
// OR based logic, in case there are multiple options defined
// the filter will return true if anyone of them is met.
func kaPolicyFilter(policy policyType.KubeArmorPolicy, o Options) bool {
	// If no filters are provided, return true
	if o.Namespace == "" && o.Labels == "" && o.Fromsource == "" && !o.IncludeNetwork {
		return true
	}

	// Namespace filtering
	if o.Namespace != "" {
		if policy.Metadata.Namespace == o.Namespace {
			log.Infof("Found namespace '%s'", o.Namespace)
			return true
		} else {
			log.Infof("Looking for '%s'.\n", o.Namespace)
		}
	}

	// Label filtering
	if o.Labels != "" {
		labelMatched := false
		providedLabels := strings.Split(o.Labels, ",")
		for _, label := range providedLabels {
			keyVal := strings.Split(label, "=")
			if len(keyVal) == 2 {
				if policyValue, exists := policy.Metadata.Labels[keyVal[0]]; exists && policyValue == keyVal[1] {
					labelMatched = true
					break
				}
			}
		}
		if labelMatched {
			return true
		} else {
			log.Info("Provided labels may not exists, please check again.")
		}
	}

	// FromSource filtering
	if o.Fromsource != "" {
		sourceMatched := false
		for _, path := range policy.Spec.Process.MatchPaths {
			for _, fromSource := range path.FromSource {
				if fromSource.Path == o.Fromsource || fromSource.Dir == o.Fromsource {
					sourceMatched = true
					break
				}
			}
		}
		for _, dir := range policy.Spec.File.MatchDirectories {
			for _, fromSource := range dir.FromSource {
				if fromSource.Path == o.Fromsource || fromSource.Dir == o.Fromsource {
					sourceMatched = true
					break
				}
			}
		}
		if sourceMatched {
			return true
		} else {
			log.Infof("FromSource '%s' not found.\n", o.Fromsource)
		}
	}

	// IncludeNetwork filtering
	if o.IncludeNetwork && len(policy.Spec.Network.MatchProtocols) > 0 {
		return true
	} else if o.IncludeNetwork {
		log.Info("No network match protocols found.")
	}

	// If none of the criteria are met
	return false
}

func networkPolicyFilter(policy policyType.KnoxNetworkPolicy, o Options) bool {
	// Metadata Label filtering
	if o.Labels != "" {
		providedLabels := strings.Split(o.Labels, ",")
		for _, label := range providedLabels {
			keyVal := strings.Split(label, "=")
			if len(keyVal) == 2 {
				if policyValue, exists := policy.Metadata[keyVal[0]]; exists && policyValue == keyVal[1] {
					return true
				}
			}
		}
	}

	// Namespace filtering from Metadata
	if o.Namespace != "" && policy.Metadata["namespace"] == o.Namespace {
		return true
	}

	// If none of the criteria are met
	return false
}

func formatPolicy(policy interface{}, o Options) (string, error) {
	var formattedPolicy string
	if o.Format == FmtJSON {
		arr, err := json.MarshalIndent(policy, "", "    ")
		if err != nil {
			return "", err
		}
		formattedPolicy = string(arr)
	} else if o.Format == FmtYAML {
		arr, err := json.Marshal(policy)
		if err != nil {
			return "", err
		}
		yamlArr, err := yaml.JSONToYAML(arr)
		if err != nil {
			return "", err
		}
		formattedPolicy = string(yamlArr)
	} else {
		return "", errors.New("only JSON and YAML formatting supported")
	}

	var metadata string
	if kPolicy, ok := policy.(policyType.KubeArmorPolicy); ok {
		metadata = fmt.Sprintf("Name:%s|Namespace:%s|Kind:KubeArmorPolicy|", kPolicy.Metadata.Name, kPolicy.Metadata.Namespace)
	}

	if nPolicy, ok := policy.(policyType.KnoxNetworkPolicy); ok {
		metadata = fmt.Sprintf("Name:%s|Namespace:%s|Kind:NetworkPolicy|", nPolicy.Metadata["name"], nPolicy.Metadata["namespace"])
	}

	return metadata + formattedPolicy, nil
}

func prettifyPolicy(formattedPolicy string, policyNumber int, totalPolicies int) string {
	parts := strings.SplitN(formattedPolicy, "|", 4)
	name := strings.Split(parts[0], ":")[1]
	namespace := strings.Split(parts[1], ":")[1]
	kind := strings.Split(parts[2], ":")[1]
	actualPolicy := parts[3]

	lines := strings.Split(actualPolicy, "\n")
	maxLength := 0
	for _, line := range lines {
		if len(line) > maxLength {
			maxLength = len(line)
		}
	}

	maxBoundaryLength := 165
	if maxLength > maxBoundaryLength {
		maxLength = maxBoundaryLength
	}

	accuKnox := fmt.Sprintf("[%d/%d] AccuKnox", policyNumber, totalPolicies)
	padding := (maxLength - len(accuKnox)) / 2
	topSeparator := "\033[36m" + strings.Repeat("=", padding) + accuKnox + strings.Repeat("=", padding) + "\033[0m"
	metaSeparator := "\033[36m" + strings.Repeat("-", maxLength) + "\033[0m"
	bottomSeparator := "\033[36m" + strings.Repeat("=", maxLength) + "\033[0m"

	prettyOutput := topSeparator + "\n"
	prettyOutput += fmt.Sprintf("\033[36mName:\033[0m      %s\n", name)
	prettyOutput += fmt.Sprintf("\033[36mNamespace:\033[0m %s\n", namespace)
	prettyOutput += fmt.Sprintf("\033[36mKind:\033[0m      %s\n", kind) + "\n"
	prettyOutput += metaSeparator + "\n\n"
	prettyOutput += actualPolicy
	prettyOutput += bottomSeparator + "\n"

	return prettyOutput
}
