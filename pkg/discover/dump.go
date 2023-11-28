package discover

import (
	"fmt"
	"os"
	"path/filepath"

	policyType "github.com/accuknox/dev2/discover/pkg/common"
	networkingv1 "k8s.io/api/networking/v1"
)

func dump(pf *PolicyForest) error {
	baseDir := "knoxctl_out/discovered/policies"
	if err := os.MkdirAll(baseDir, os.ModePerm); err != nil {
		return fmt.Errorf("failed to create base directory %s: %v", baseDir, err)
	}

	allPolicies := pf.GetAllPolicies()

	for ns, nsPolicies := range allPolicies {
		kubearmorPolicies := nsPolicies["KubearmorPolicies"].([]*policyType.KubeArmorPolicy)
		networkPolicies := nsPolicies["NetworkPolicies"].([]*networkingv1.NetworkPolicy)

		for _, policy := range kubearmorPolicies {
			kubearmorDir := filepath.Join(baseDir, "kubearmor_policy", ns)
			if err := os.MkdirAll(kubearmorDir, os.ModePerm); err != nil {
				return fmt.Errorf("failed to create directory %s: %v", kubearmorDir, err)
			}

			filename := fmt.Sprintf("%s.yaml", policy.Metadata.Name)

			if err := writePolicyToFile(policy, kubearmorDir, filename); err != nil {
				return fmt.Errorf("failed to write policy to file %s: %v", filename, err)
			}
		}

		for _, policy := range networkPolicies {
			networkDir := filepath.Join(baseDir, "network_policy", ns)
			if err := os.MkdirAll(networkDir, os.ModePerm); err != nil {
				return fmt.Errorf("failed to create directory %s: %v", networkDir, err)
			}

			filename := fmt.Sprintf("%s.yaml", policy.Name)

			if err := writeNetworkPolicyToFile(policy, networkDir, filename); err != nil {
				return fmt.Errorf("failed to write policy to file %s: %v", filename, err)
			}
		}
	}

	return nil
}

func writePolicyToFile(policy *policyType.KubeArmorPolicy, nsDirPath, filename string) error {
	yamlStr := kubearmorPolicyToString(policy)

	filePath := filepath.Join(nsDirPath, filename)
	return os.WriteFile(filePath, []byte(yamlStr), 0644)
}

func writeNetworkPolicyToFile(policy *networkingv1.NetworkPolicy, nsDirPath, filename string) error {
	yamlStr := networkPolicyToString(policy)

	filePath := filepath.Join(nsDirPath, filename)
	return os.WriteFile(filePath, []byte(yamlStr), 0644)
}
