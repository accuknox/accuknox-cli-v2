package recommend

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/kubearmor/kubearmor-client/k8s"
	"github.com/olekukonko/tablewriter"
)

func printJSON(pb *PolicyBucket) {
	for _, ab := range pb.Namespaces {
		policies := getAllPoliciesInBucket(ab)
		if len(policies) == 0 {
			continue
		}

		jsonData, err := json.MarshalIndent(policies, "", "  ")
		if err != nil {
			fmt.Printf("failed to marshal policies: %v\n", err)
			continue
		}
		fmt.Println(string(jsonData))
	}
}

func printYAML(pb *PolicyBucket) {
	for _, ab := range pb.Namespaces {
		policies := getAllPoliciesInBucket(ab)
		if len(policies) == 0 {
			continue
		}

		for _, policy := range policies {
			yamlData := policyToString(policy)
			fmt.Println(yamlData)
		}
	}
}

func printTable(pb *PolicyBucket, o *Options, c *k8s.Client) {
	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader([]string{"Name", "Namespace", "Labels", "Tags", "Severity", "Action", "TLDR"})
	table.SetRowLine(true)

	for ns := range pb.Namespaces {
		policies, err := pb.RetrievePolicies(c, o)
		if err != nil {
			fmt.Printf("failed to retrieve policies: %v\n", err)
			continue
		}

		for _, policy := range policies {
			name := strings.ReplaceAll(policy.Metadata.Name, "-", "-\n")
			labels := formatLabels(policy.Spec.Selector.MatchLabels)
			tags := strings.Join(policy.Spec.Tags, ", ")
			severity := fmt.Sprintf("%d", policy.Spec.Severity)
			action := policy.Spec.Action
			tldr := policy.Metadata.Annotations["app.accuknox.com/tldr"]

			if tldr == "" {
				tldr = "N/A"
			}

			table.Append([]string{name, ns, labels, tags, severity, action, tldr})
		}
	}

	table.Render()
}

func formatLabels(labels map[string]string) string {
	var sb strings.Builder
	for k, v := range labels {
		sb.WriteString(k)
		sb.WriteString("=")
		sb.WriteString(v)
		sb.WriteString("\n")
	}

	return sb.String()
}
