package cmd

import (
	"github.com/accuknox/accuknox-cli-v2/pkg/api/cluster"
	"github.com/accuknox/accuknox-cli-v2/pkg/config"
	"github.com/accuknox/accuknox-cli-v2/pkg/logger"
	"github.com/spf13/cobra"
)

var clusterListOptions cluster.CLusterListOptions

// clusterListCmd represents the `list` subcommand for clusters
var clusterListCmd = &cobra.Command{
	Use:     "list",
	Short:   "List clusters and its relevant information and its corresponding entities (e.g., nodes)",
	Long:    `The 'cluster list' command retrieves a list of onboarded clusters and optionally displays additional details like nodes within each cluster.`,
	Example: cluster.ClusterListDescription,
	Run: func(cmd *cobra.Command, args []string) {
		if err := config.LoadConfig(CFG_FILE); err != nil {
			logger.Error(err.Error())
			return
		}
		config.SetConfig(CWPP_URL, CSPM_URL, TOKEN, TENANT_ID)
		cluster.FetchClusterInfo(clusterListOptions)
	},
}

func init() {
	clusterCmd.AddCommand(clusterListCmd)

	clusterListCmd.Flags().StringVar(&clusterListOptions.ClusterListJQ, "clusterjq", ".[]", "JQ filter to apply on cluster list output")
	clusterListCmd.Flags().StringVar(&clusterListOptions.NodeJQ, "nodejq", ".result[]", "JQ filter to apply on node list output")
	clusterListCmd.Flags().BoolVar(&clusterListOptions.ShowNodes, "nodes", false, "Display nodes across all the clusters")
	clusterListCmd.Flags().BoolVar(&clusterListOptions.NoPager, "noPager", false, "Dumps complete cluster list")
	clusterListCmd.Flags().StringVar(&clusterListOptions.ClusterName, "clusterName", "", "List nodes based on cluster name")
	clusterListCmd.Flags().BoolVar(&clusterListOptions.JsonFormat, "json", false, "Flag to list cluster and nodes in the JSON format")
	clusterListCmd.Flags().IntVar(&clusterListOptions.Page, "page", 0, "Page number for alerts listing")
	clusterListCmd.Flags().IntVar(&clusterListOptions.PageSize, "page-size", 50, "Number of alerts to list per page")
}
