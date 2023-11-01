// Package discover fetches discovered policies from discovery engine
package discover

import (
	"errors"
	"fmt"
	"strings"

	"github.com/kubearmor/kubearmor-client/k8s"

	log "github.com/sirupsen/logrus"
)

const (
	KindK8sNetworkPolicy    = "NetworkPolicy"
	KindKubeArmorPolicy     = "KubeArmorPolicy"
	KindKubeArmorHostPolicy = "KubeArmorHostPolicy"

	PolicyType = "discovered"

	FmtYAML = "yaml"
	FmtJSON = "json"
)

type policyHandler struct {
	fn func(*k8s.Client, *Options) ([]string, error)
}

func Policy(c *k8s.Client, parsedArgs *Options) error {
	defer disconnect()
	fmt.Println("Discovering policies...")

	log.Infof("Parsed Args: %+v", parsedArgs)
	policies := getSupportedPolicies()
	toProcess, err := determinePoliciesToProcess(parsedArgs, policies)
	if err != nil {
		return err
	}

	errorSlice := []string{}
	for kind, process := range toProcess {
		if !process {
			continue
		}
		data, err := fetchPolicyData(policies, kind, c, parsedArgs)
		if err != nil {
			log.WithFields(log.Fields{
				"kind":           parsedArgs.Kind,
				"format":         parsedArgs.Format,
				"namespace":      parsedArgs.Namespace,
				"labels":         parsedArgs.Labels,
				"fromSource":     parsedArgs.Source,
				"gRPC":           parsedArgs.GRPC,
				"includeNetwork": parsedArgs.IncludeNetwork,
			}).Warn("failed to process/fetch policies")
			errorSlice = append(errorSlice, err.Error())
			continue
		}

		pd := NewPolicyDisplay(data)
		if err := pd.Display(*parsedArgs); err != nil {
			errorSlice = append(errorSlice, err.Error())
		}
	}

	if len(errorSlice) > 0 {
		return errors.New(strings.Join(errorSlice, "; "))
	}

	return nil
}

// As we support more type of policies in future we can extend here
func getSupportedPolicies() map[string]policyHandler {
	return map[string]policyHandler{
		KindK8sNetworkPolicy:    {getNetworkPolicy},
		KindKubeArmorHostPolicy: {getKaHostPolicy},
		KindKubeArmorPolicy:     {getKaPolicy},
	}
}

func determinePoliciesToProcess(parsedArgs *Options, policies map[string]policyHandler) (map[string]bool, error) {
	toProcess := make(map[string]bool)

	var supportedPolicies []string
	for k := range policies {
		supportedPolicies = append(supportedPolicies, k)
		toProcess[k] = false
	}

	if len(parsedArgs.Kind) == 0 {
		toProcess[KindKubeArmorPolicy] = true
		return toProcess, nil
	}

	for _, kind := range parsedArgs.Kind {
		if _, exists := policies[kind]; exists {
			toProcess[kind] = true
		} else {
			return nil, fmt.Errorf("unsupported policy: %s. Supported policies: %s", kind, strings.Join(supportedPolicies, ", "))
		}
	}

	return toProcess, nil
}

func fetchPolicyData(policies map[string]policyHandler, kind string, c *k8s.Client, p *Options) ([]string, error) {
	data, err := policies[kind].fn(c, p)
	if err != nil {
		return nil, err
	}

	totalPolicies := len(data)
	log.Infof("Total policies discovered by the discovery engine: [%v]", totalPolicies)
	return data, nil
}
