// Package discover fetches discovered policies from discovery engine
package discover

import (
	"errors"
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

// Options Structure
type Options struct {
	GRPC           string
	Format         string
	Kind           string
	Namespace      string
	Labels         string
	Fromsource     string
	IncludeNetwork bool
}

type policyHandler struct {
	fn func(*k8s.Client, Options) ([]string, error)
}

func Policy(c *k8s.Client, o Options) error {
	defer disconnect()
	log.Info("Discovering policies...")

	policies := getSupportedPolicies()
	toProcess, err := determinePoliciesToProcess(o, policies)
	if err != nil {
		return err
	}

	var errorSlice []string
	for kind, process := range toProcess {
		if process {
			data, err := fetchPolicyData(policies, kind, c, o)
			if err != nil {
				log.WithFields(log.Fields{
					"kind":           o.Kind,
					"format":         o.Format,
					"namespace":      o.Namespace,
					"labels":         o.Labels,
					"fromSource":     o.Fromsource,
					"gRPC":           o.GRPC,
					"includeNetwork": o.IncludeNetwork,
				}).Warn("failed to process/fetch policies")
				errorSlice = append(errorSlice, err.Error())
				continue
			}

			pd := NewPolicyDisplay(data)
			err = pd.Display(o)
			if err != nil {
				errorSlice = append(errorSlice, err.Error())
			}
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

func determinePoliciesToProcess(o Options, policies map[string]policyHandler) (map[string]bool, error) {
	toProcess := make(map[string]bool)
	for k := range policies {
		toProcess[k] = true
	}

	if o.Kind != "" {
		if _, exists := policies[o.Kind]; !exists {
			var supportedPolicies []string
			for policyKey := range toProcess {
				supportedPolicies = append(supportedPolicies, policyKey)
			}
			return nil, errors.New("the policy you are requesting is not supported. \nCurrently supported policies are: " + strings.Join(supportedPolicies, ", "))
		}

		for k := range toProcess {
			if k != o.Kind {
				toProcess[k] = false
			}
		}
	}
	return toProcess, nil
}

func fetchPolicyData(policies map[string]policyHandler, kind string, c *k8s.Client, o Options) ([]string, error) {
	data, err := policies[kind].fn(c, o)
	if err != nil {
		return nil, err
	}

	totalPolicies := len(data)
	log.Infof("Total policies discovered by the discovery engine: [%v]", totalPolicies)
	return data, nil
}
