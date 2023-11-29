// Package discover fetches discovered policies from discovery engine
package discover

import (
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/kubearmor/kubearmor-client/k8s"
)

const (
	KindK8sNetworkPolicy    = "NetworkPolicy"
	KindKubeArmorPolicy     = "KubeArmorPolicy"
	KindKubeArmorHostPolicy = "KubeArmorHostPolicy"

	PolicyType = "discovered"

	FmtYAML  = "yaml"
	FmtJSON  = "json"
	FmtTable = "table"
)

type policyHandler struct {
	fn func(*k8s.Client, *Options, *PolicyForest) error
}

func Policy(c *k8s.Client, parsedArgs *Options) error {
	defer disconnect()
	fmt.Println("Discovering policies...")

	err := initConnection(c, parsedArgs)
	if err != nil {
		return err
	}

	policies := getSupportedPolicies()
	toProcess, err := determinePoliciesToProcess(parsedArgs, policies)
	if err != nil {
		return err
	}

	policyForest := NewPolicyForest()
	var wg sync.WaitGroup

	errorChan := make(chan error, len(toProcess))

	for kind, process := range toProcess {
		if process {
			wg.Add(1)

			go func(kind string, handler policyHandler) {
				defer wg.Done()

				err := handler.fn(c, parsedArgs, policyForest)
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

	wg.Wait()

	if len(policyForest.Namespaces) != 0 {
		switch {
		case parsedArgs.View == FmtYAML:
			printYAML(policyForest)

		case parsedArgs.View == FmtJSON:
			printJSON(policyForest)

		case parsedArgs.View == FmtTable:
			printTable(policyForest)

		case parsedArgs.Dump:
			err := dump(policyForest)
			if err != nil {
				return fmt.Errorf("failed to dump policies: %v", err)
			}

		default:
			StartTUI(policyForest)
		}
	} else {
		fmt.Println("No policies found.")
	}

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
