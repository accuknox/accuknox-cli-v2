package report

import (
	"fmt"
	"time"

	"github.com/accuknox/accuknox-cli-v2/pkg/common"
	"github.com/accuknox/accuknox-cli-v2/pkg/summary"
	"github.com/clarketm/json"
	"github.com/kubearmor/kubearmor-client/k8s"
)

type outputPaths struct {
	Level4NodesOutput string
	PrMDOutput        string
	DiffOutput        string
	LatestSummaryPath string
}

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
	currentTime := time.Now().UTC().Format("20060102-150405")
	outputPaths := generateOutputPaths(o.OutputTo, currentTime)

	err = writeLatestSummary(outputPaths.LatestSummaryPath, latestSummary)
	if err != nil {
		return err
	}

	err = tracker.markdownPR(outputPaths.PrMDOutput, latestSummary.GetHash())
	if err != nil {
		return err
	}

	err = tracker.writeDiffJSON(outputPaths.DiffOutput, latestSummary.GetHash())
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

		err = tracker.writeLevel4NodesToJSONFile(outputPaths.Level4NodesOutput, latestSummary.GetHash())
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

func writeLatestSummary(outpath string, workload *summary.Workload) error {
	jsonData, err := json.MarshalIndent(workload, "", "    ")
	if err != nil {
		return err
	}

	err = common.CleanAndWrite(outpath, jsonData)
	if err != nil {
		return err
	}

	fmt.Printf("Latest summary (in json) written to: %s\n", outpath)
	return nil
}

func generateOutputPaths(outputDir, currentTime string) outputPaths {
	basePath := "knoxctl_out/reports/"
	if outputDir != "" {
		basePath = outputDir
	}

	return outputPaths{
		Level4NodesOutput: fmt.Sprintf("%slevel_4_nodes_%s.json", basePath, currentTime),
		PrMDOutput:        fmt.Sprintf("%spr_report_%s.md", basePath, currentTime),
		DiffOutput:        fmt.Sprintf("%sdiff_%s.json", basePath, currentTime),
		LatestSummaryPath: fmt.Sprintf("%slatest_summary_%s.json", basePath, currentTime),
	}
}
