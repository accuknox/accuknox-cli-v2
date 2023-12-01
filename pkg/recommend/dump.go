package recommend

import (
	"fmt"
	"os"
	"path/filepath"

	policyType "github.com/accuknox/dev2/hardening/pkg/types"
)

func dump(pb *PolicyBucket) error {
	dirPath := "knoxctl_out/recommended/policies"
	if err := os.MkdirAll(dirPath, os.ModePerm); err != nil {
		return fmt.Errorf("failed to create directory: %v", err)
	}

	for _, ab := range pb.Namespaces {
		policies := getAllPoliciesInBucket(ab)

		for _, policy := range policies {
			ns := policy.Metadata.Namespace

			nsDirPath := filepath.Join(dirPath, ns)
			if err := os.MkdirAll(nsDirPath, os.ModePerm); err != nil {
				return fmt.Errorf("failed to create namespace directory '%s': %v", nsDirPath, err)
			}

			filename := fmt.Sprintf("%s-%s.yaml", ns, policy.Metadata.Name)
			if err := writeKubearmorPolicyToFile(policy, nsDirPath, filename); err != nil {
				continue
			}
		}
	}

	return nil
}

func writeKubearmorPolicyToFile(policy *policyType.KubeArmorPolicy, nsDirPath, filename string) error {
	yamlStr := policyToString(policy)
	filePath := filepath.Join(nsDirPath, filename)
	return os.WriteFile(filePath, []byte(yamlStr), 0600)
}
