package discover

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/accuknox/accuknox-cli-v2/pkg/common"
	"github.com/clarketm/json"
	"github.com/kubearmor/kubearmor-client/k8s"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"sigs.k8s.io/yaml"

	dev2policy "github.com/accuknox/dev2/api/grpc/v1/policy"
	policyType "github.com/accuknox/dev2/discover/pkg/common"
	log "github.com/sirupsen/logrus"
)

var connection *grpc.ClientConn

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

func getNetworkPolicy(c *k8s.Client, p *Options) ([]string, error) {
	var data []string

	gRPC, err := common.ConnectGrpc(c, p.GRPC)
	if err != nil {
		log.WithError(err).Error("failed to initialize gRPC connection")
		return nil, err
	}
	connection, err = grpc.Dial(gRPC, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.WithError(err).Error("failed to connect to discovery engine")
		return nil, err
	}

	client := dev2policy.NewGetPolicyClient(connection)
	resp, err := client.GetPolicy(context.Background(), &dev2policy.PolicyRequest{
		Type: PolicyType,           // discovered
		Kind: KindK8sNetworkPolicy, // NetworkPolicy
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

			if !networkPolicyFilter(networkPolicy, p) {
				continue
			}

			formattedPolicy, err := formatPolicy(networkPolicy, p)
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

func getKaHostPolicy(c *k8s.Client, p *Options) ([]string, error) {
	var data []string

	gRPC, err := common.ConnectGrpc(c, p.GRPC)
	if err != nil {
		log.WithError(err).Error("failed to initialize gRPC connection")
		return nil, err
	}
	connection, err = grpc.Dial(gRPC, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.WithError(err).Error("failed to connect to discovery engine")
		return nil, err
	}

	client := dev2policy.NewGetPolicyClient(connection)
	resp, err := client.GetPolicy(context.Background(), &dev2policy.PolicyRequest{
		Type: PolicyType,              // discovered
		Kind: KindKubeArmorHostPolicy, // KAHostPolicy
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

			if !kaPolicyFilter(kaHostPolicy, p) {
				continue
			}

			formattedPolicy, err := formatPolicy(kaHostPolicy, p)
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

func getKaPolicy(c *k8s.Client, p *Options) ([]string, error) {
	var data []string

	gRPC, err := common.ConnectGrpc(c, p.GRPC)
	if err != nil {
		log.WithError(err).Error("failed to initialize gRPC connection")
		return nil, err
	}
	connection, err = grpc.Dial(gRPC, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.WithError(err).Error("failed to connect to discovery engine")
		return nil, err
	}

	client := dev2policy.NewGetPolicyClient(connection)
	resp, err := client.GetPolicy(context.Background(), &dev2policy.PolicyRequest{
		Type: PolicyType,          // discovered
		Kind: KindKubeArmorPolicy, // KAPolicy
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

			if !kaPolicyFilter(kaPolicy, p) {
				continue
			}

			formattedPolicy, err := formatPolicy(kaPolicy, p)
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

// Centralized filteration based on user options
// OR based filter at flag level and AND based filter at command level.
func kaPolicyFilter(policy policyType.KubeArmorPolicy, p *Options) bool {
	// If no filters are provided, return true
	if p.noFilters() {
		return true
	}

	namespaceMatched := len(p.Namespace) == 0 && p.NamespaceRegex == nil
	labelMatched := len(p.Labels) == 0 && p.LabelsRegex == nil
	sourceMatched := len(p.Source) == 0 && p.SourceRegex == nil
	includeNetworkMatched := !p.IncludeNetwork || len(policy.Spec.Network.MatchProtocols) > 0

	// Namespace filtering
	if !namespaceMatched {
		for _, ns := range p.Namespace {
			if policy.Metadata.Namespace == ns {
				log.Infof("Found namespace '%s'", ns)
				namespaceMatched = true
				break
			}
		}
		if p.NamespaceRegex != nil && !namespaceMatched {
			for _, regex := range p.NamespaceRegex {
				if regex.MatchString(policy.Metadata.Namespace) {
					log.Infof("Namespace matched by regex")
					namespaceMatched = true
					break
				}
			}
		}
	}

	// Labels filtering
	if !labelMatched {
		for _, label := range p.Labels {
			keyVal := strings.Split(label, "=")
			if len(keyVal) == 2 {
				if policyValue, exists := policy.Spec.Selector.MatchLabels[keyVal[0]]; exists && policyValue == keyVal[1] {
					log.Info("Label matched")
					labelMatched = true
					break
				}
			}
		}
		if p.LabelsRegex != nil && !labelMatched {
			for _, regex := range p.LabelsRegex {
				for k, v := range policy.Spec.Selector.MatchLabels {
					if regex.MatchString(k + "=" + v) {
						log.Info("Label matched by regex")
						labelMatched = true
						break
					}
				}
				if labelMatched {
					break
				}
			}
		}
	}

	// FromSource filtering
	if !sourceMatched {
		for _, path := range policy.Spec.Process.MatchPaths {
			for _, fromSource := range path.FromSource {
				for _, src := range p.Source {
					if fromSource.Path == src || fromSource.Dir == src {
						sourceMatched = true
						break
					}
				}
				if sourceMatched {
					break
				}
				if p.SourceRegex != nil {
					for _, regex := range p.SourceRegex {
						if regex.MatchString(fromSource.Path) || regex.MatchString(fromSource.Dir) {
							sourceMatched = true
							break
						}
					}
				}
				if sourceMatched {
					break
				}
			}
			if sourceMatched {
				break
			}
		}
		if !sourceMatched {
			for _, dir := range policy.Spec.File.MatchDirectories {
				for _, fromSource := range dir.FromSource {
					for _, src := range p.Source {
						if fromSource.Path == src || fromSource.Dir == src {
							sourceMatched = true
							break
						}
					}
					if sourceMatched {
						break
					}
					if p.SourceRegex != nil {
						for _, regex := range p.SourceRegex {
							if regex.MatchString(fromSource.Path) || regex.MatchString(fromSource.Dir) {
								sourceMatched = true
								break
							}
						}
					}
					if sourceMatched {
						break
					}
				}
				if sourceMatched {
					break
				}
			}
		}
		if sourceMatched {
			log.Infof("FromSource matched")
		} else {
			log.Infof("FromSource '%s' not found.", strings.Join(p.Source, ","))
		}
	}

	return namespaceMatched && labelMatched && sourceMatched && includeNetworkMatched
}

func formatPolicy(policy interface{}, p *Options) (string, error) {
	if p.Format == "" {
		p.Format = "yaml"
	}

	var formattedPolicy string
	if p.Format == FmtJSON {
		arr, err := json.MarshalIndent(policy, "", "    ")
		if err != nil {
			return "", err
		}
		formattedPolicy = string(arr)
	} else if p.Format == FmtYAML {
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

func networkPolicyFilter(policy policyType.KnoxNetworkPolicy, p *Options) bool {
	if p.noFilters() {
		return true
	}

	labelMatched := len(p.Labels) == 0 && p.LabelsRegex == nil
	namespaceMatched := len(p.Namespace) == 0 && p.NamespaceRegex == nil

	// Metadata Label filtering
	if !labelMatched {
		for _, label := range p.Labels {
			keyVal := strings.Split(label, "=")
			if len(keyVal) == 2 {
				if policyValue, exists := policy.Metadata[keyVal[0]]; exists && policyValue == keyVal[1] {
					labelMatched = true
					break
				}
			}
		}
		if p.LabelsRegex != nil && !labelMatched {
			for _, regex := range p.LabelsRegex {
				for key, value := range policy.Metadata {
					if regex.MatchString(key + "=" + value) {
						labelMatched = true
						break
					}
				}
				if labelMatched {
					break
				}
			}
		}
	}

	// Namespace filtering from Metadata
	if !namespaceMatched {
		for _, ns := range p.Namespace {
			if policy.Metadata["namespace"] == ns {
				namespaceMatched = true
				break
			}
		}
		if p.NamespaceRegex != nil && !namespaceMatched {
			for _, regex := range p.NamespaceRegex {
				if regex.MatchString(policy.Metadata["namespace"]) {
					namespaceMatched = true
					break
				}
			}
		}
	}

	// If both criteria are met
	return labelMatched && namespaceMatched
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
