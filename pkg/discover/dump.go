package discover

import (
	"fmt"
	"os"
	"path/filepath"

	policyType "github.com/accuknox/dev2/discover/pkg/common"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/klog/v2"
)

func dump(forest *PolicyForest) error {
	dirPath := "knoxctl_out"
	if err := os.MkdirAll(dirPath, os.ModePerm); err != nil {
		return fmt.Errorf("failed to create directory: %v", err)
	}

	klog.Info("Len of forest.Namespaces: ", len(forest.Namespaces))
	for ns, nsBucket := range forest.Namespaces {
		nsDirPath := filepath.Join(dirPath, ns)
		if err := os.MkdirAll(nsDirPath, os.ModePerm); err != nil {
			return fmt.Errorf("failed to create namespace directory '%s': %v", nsDirPath, err)
		}

		for _, policies := range nsBucket.KubearmorPolicies.Labels {
			for _, policy := range policies {
				filename := fmt.Sprintf("%s-KAP-Label-%s.yaml", ns, policy.Metadata.Name)
				if err := writePolicyToFile(policy, nsDirPath, filename); err != nil {
					continue
				}
			}
		}

		// Handle KubeArmorPolicies in Actions
		for _, policies := range nsBucket.KubearmorPolicies.Actions {
			for _, policy := range policies {
				filename := fmt.Sprintf("%s-KAP-Action-%s.yaml", ns, policy.Metadata.Name)
				if err := writePolicyToFile(policy, nsDirPath, filename); err != nil {
					continue
				}
			}
		}

		// Handle NetworkPolicies in Types
		for typeKey, policies := range nsBucket.NetworkPolicies.Types {
			for _, policy := range policies {
				filename := fmt.Sprintf("%s-NP-Type-%s-%s.yaml", ns, typeKey, policy.ObjectMeta.Name)
				if err := writeNetworkPolicyToFile(policy, nsDirPath, filename); err != nil {
					continue
				}
			}
		}
		// Handle NetworkPolicies in Protocols
		for protocolKey, policies := range nsBucket.NetworkPolicies.Protocols {
			for _, policy := range policies {
				filename := fmt.Sprintf("%s-NP-Protocol-%s-%s.yaml", ns, protocolKey, policy.ObjectMeta.Name)
				if err := writeNetworkPolicyToFile(policy, nsDirPath, filename); err != nil {
					continue
				}
			}
		}
	}

	return nil
}

func writePolicyToFile(policy *policyType.KubeArmorPolicy, nsDirPath, filename string) error {
	yamlStr := kubearmorPolicyToString(policy)

	fmt.Println(yamlStr) // Print to terminal

	filePath := filepath.Join(nsDirPath, filename)
	return os.WriteFile(filePath, []byte(yamlStr), 0644)
}

func writeNetworkPolicyToFile(policy *networkingv1.NetworkPolicy, nsDirPath, filename string) error {
	yamlStr := networkPolicyToString(policy)

	fmt.Println(yamlStr) // Print to terminal

	filePath := filepath.Join(nsDirPath, filename)
	return os.WriteFile(filePath, []byte(yamlStr+"---\n"), 0644) // Appending '---\n' at the end of each policy
}
