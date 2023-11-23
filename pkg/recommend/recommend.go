package recommend

import (
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/kubearmor/kubearmor-client/k8s"
)

const (
	KindKubeArmorPolicy = "KubeArmorPolicy"

	PolicyType = "hardening"
)

type policyHandler struct {
	fn func(*k8s.Client, *Options) ([]string, error)
}

func Recommend(c *k8s.Client, o *Options) error {
	policies := getSupportedPolicies()
	toProcess, err := determinePoliciesToProcess(o, policies)
	if err != nil {
		return err
	}

	var wg sync.WaitGroup
	errorChan := make(chan error, len(toProcess))

	for kind, process := range toProcess {
		if process {
			wg.Add(1)
			go func(kind string, handler policyHandler) {
				defer wg.Done()
				_, err := fetchPolicyData(policies, kind, c, o)
				if err != nil {
					errorChan <- err
				}
			}(kind, policies[kind])
		}
	}

	go func() {
		wg.Wait()
		close(errorChan)
	}()

	var errorSlice []string
	for err := range errorChan {
		if err != nil {
			errorSlice = append(errorSlice, err.Error())
		}
	}

	if len(errorSlice) > 0 {
		return errors.New(strings.Join(errorSlice, "; "))
	}

	return nil
}

func getSupportedPolicies() map[string]policyHandler {
	return map[string]policyHandler{
		KindKubeArmorPolicy: {getKaPolicy},
	}
}

func determinePoliciesToProcess(o *Options, policies map[string]policyHandler) (map[string]bool, error) {
	toProcess := make(map[string]bool)

	var supportedPolicies []string
	for k := range policies {
		supportedPolicies = append(supportedPolicies, k)
		toProcess[k] = false
	}

	if len(o.Policy) == 0 {
		toProcess[KindKubeArmorPolicy] = true
		return toProcess, nil
	}

	for _, kind := range o.Policy {
		if _, exists := policies[kind]; exists {
			toProcess[kind] = true
		} else {
			return nil, fmt.Errorf("unsupported policy: %s, currently supported policies are: %s", kind, strings.Join(supportedPolicies, ", "))
		}
	}

	return toProcess, nil
}

func fetchPolicyData(policies map[string]policyHandler, kind string, c *k8s.Client, o *Options) ([]string, error) {
	data, err := policies[kind].fn(c, o)
	if err != nil {
		return nil, err
	}

	return data, nil
}
