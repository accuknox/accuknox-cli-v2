package discover

import (
	"fmt"
	"os"
	"strings"

	"github.com/clarketm/json"
	"github.com/olekukonko/tablewriter"

	policyType "github.com/accuknox/dev2/discover/pkg/common"
	networkingv1 "k8s.io/api/networking/v1"
)

func printJSON(pf *PolicyForest) {
	allPolicies := pf.GetAllPolicies()

	for ns, policies := range allPolicies {
		if len(policies) == 0 {
			continue
		}

		fmt.Printf("Namespace: %s\n", ns)

		jsonData, err := json.MarshalIndent(policies, "", "  ")
		if err != nil {
			fmt.Printf("failed to marshal policies: %v\n", err)
			continue
		}

		fmt.Println(string(jsonData))
	}
}

func printYAML(pf *PolicyForest) {
	allPolicies := pf.GetAllPolicies()

	for _, policies := range allPolicies {
		if len(policies) == 0 {
			continue
		}

		for _, policy := range policies["KubearmorPolicies"].([]*policyType.KubeArmorPolicy) {
			yamlData := kubearmorPolicyToString(policy)
			fmt.Println(yamlData)
		}

		for _, policy := range policies["NetworkPolicies"].([]*networkingv1.NetworkPolicy) {
			yamlData := networkPolicyToString(policy)
			fmt.Println(yamlData)
		}

		fmt.Println(strings.Repeat("-", 3))
	}
}

func printTable(pf *PolicyForest) {
	allPolicies := pf.GetAllPolicies()

	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader([]string{"Namespace", "Policy Name", "Type", "Policy (yaml)"})
	table.SetAutoWrapText(false)

	for namespace, nsPolicies := range allPolicies {
		for _, policy := range nsPolicies["KubearmorPolicies"].([]*policyType.KubeArmorPolicy) {
			table.Append([]string{namespace, policy.Metadata.Name, "KubeArmor", kubearmorPolicyToString(policy)})
		}

		for _, policy := range nsPolicies["NetworkPolicies"].([]*networkingv1.NetworkPolicy) {
			table.Append([]string{namespace, policy.ObjectMeta.Name, "Network", networkPolicyToString(policy)})
		}
	}

	table.Render()
}
