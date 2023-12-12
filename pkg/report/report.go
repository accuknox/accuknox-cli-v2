package report

import (
	"fmt"

	"github.com/accuknox/accuknox-cli-v2/pkg/common"
	"github.com/accuknox/accuknox-cli-v2/pkg/summary"
	"github.com/clarketm/json"
	"github.com/kubearmor/kubearmor-client/k8s"
)

func Report(client *k8s.Client, o *summary.Options) error {
	if o.BaselineSummaryPath == "" {
		return fmt.Errorf("baseline summary file path is required")
	}

	fmt.Println("Getting latest summary...")

	latestSummary, err := summary.GetSummary(client, *o)
	if err != nil {
		return fmt.Errorf("failed to get summary: %v", err)
	}

	baselineSummary, err := loadBaselineSummary(o.BaselineSummaryPath)
	if err != nil {
		return fmt.Errorf("failed to load baseline summary: %v", err)
	}

	fmt.Println("Generating report...")
	tracker := NewGraph()

	Difference(latestSummary, baselineSummary, tracker)

	tracker.cancelOutChanges(latestSummary.GetHash())
	tracker.FilterGraph(latestSummary.GetHash(), o)

	// Determine output paths
	predefinedPath := "knoxctl_out/reports/"
	level4NodesOutput := predefinedPath + "level_4_nodes.json"
	testPrMdOutput := predefinedPath + "test_pr_md.md"
	diffOutput := predefinedPath + "diff.json"
	if o.OutputTo != "" {
		level4NodesOutput = o.OutputTo
		testPrMdOutput = o.OutputTo
		diffOutput = o.OutputTo
	}

	err = tracker.markdownPR(testPrMdOutput, latestSummary.GetHash())
	if err != nil {
		return err
	}

	err = tracker.writeDiffJSON(diffOutput, latestSummary.GetHash())
	if err != nil {
		return err
	}

	if o.View == "table" {
		err := tracker.printTable(latestSummary.GetHash())
		if err != nil {
			return err
		}
	}

	// for internal debugging purposes
	if o.Debug {
		tracker.printDFSTraversal(latestSummary.GetHash())

		err = tracker.writeLevel4NodesToJSONFile(level4NodesOutput, latestSummary.GetHash())
		if err != nil {
			return err
		}
	}

	return nil
}

func loadBaselineSummary(baselinePath string) (*summary.Workload, error) {
	fileContent, err := common.CleanAndRead(baselinePath)
	if err != nil {
		return nil, fmt.Errorf("error reading baseline file: %w", err)
	}

	var workload summary.Workload

	if err := json.Unmarshal(fileContent, &workload); err != nil {
		return nil, fmt.Errorf("error unmarshalling JSON: %w", err)
	}

	if err := setHashForBaseline(&workload); err != nil {
		return nil, err
	}

	return &workload, nil
}

func setHashForBaseline(workload *summary.Workload) error {
	for _, cluster := range workload.Clusters {
		for _, namespace := range cluster.Namespaces {
			if err := setHashForResource(namespace.Deployments); err != nil {
				return err
			}
			if err := setHashForResource(namespace.ReplicaSets); err != nil {
				return err
			}
			if err := setHashForResource(namespace.StatefulSets); err != nil {
				return err
			}
			if err := setHashForResource(namespace.DaemonSets); err != nil {
				return err
			}
			if err := setHashForResource(namespace.Jobs); err != nil {
				return err
			}
			if err := setHashForResource(namespace.CronJobs); err != nil {
				return err
			}
			if err := namespace.SetHash(); err != nil {
				return err
			}
		}
		if err := cluster.SetHash(); err != nil {
			return err
		}
	}
	return workload.SetHash()
}

func setHashForResource(resources map[string]*summary.WorkloadEvents) error {
	for _, resource := range resources {
		if err := resource.SetHash(); err != nil {
			return err
		}
	}
	return nil
}
