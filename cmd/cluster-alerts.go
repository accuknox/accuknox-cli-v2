package cmd

import (
	"time"

	"github.com/accuknox/accuknox-cli-v2/pkg/api/cluster"
	"github.com/accuknox/accuknox-cli-v2/pkg/config"
	"github.com/accuknox/accuknox-cli-v2/pkg/logger"
	"github.com/spf13/cobra"
)

var (
	startTime = endTime - 2*24*60*60
	endTime   = time.Now().Unix()
)

var clusterAlertsOptions cluster.ClusterALertOptions

// clusterAlertsCmd represents the `alerts` subcommand for clusters
var clusterAlertsCmd = &cobra.Command{
	Use:     "alerts",
	Short:   "Show alerts",
	Long:    `Show alerts in the context of clusters. These alerts could be from KubeArmor, Network policies, Admission controllers or anything else as reported in "Monitors & Alerts" option in AccuKnox Control Plane.`,
	Example: cluster.ClusterAlertDescription,
	Run: func(cmd *cobra.Command, args []string) {
		if err := config.LoadConfig(CFG_FILE); err != nil {
			logger.Error(err.Error())
			return
		}
		config.SetConfig(CWPP_URL, CSPM_URL, TOKEN, TENANT_ID)
		cluster.FetchClusterAlerts(clusterAlertsOptions)
	},
}

func init() {
	clusterCmd.AddCommand(clusterAlertsCmd)

	clusterAlertsCmd.Flags().StringVar(&clusterAlertsOptions.AlertType, "type", "kubearmor", "Set alert type")
	clusterAlertsCmd.Flags().StringVar(&clusterAlertsOptions.Cluster_id, "cluster-id", "", "Set Cluster ID (eg: 1234,5678,9012)")
	clusterAlertsCmd.Flags().StringVar(&clusterAlertsOptions.LogType, "log-type", "active", "Set log type [active|suppressed|all]")
	clusterAlertsCmd.Flags().StringVar(&clusterAlertsOptions.ClusterAlertJQ, "clusterjq", ".[]", "JQ filter for cluster list output")
	clusterAlertsCmd.Flags().StringVar(&clusterAlertsOptions.AlertJQ, "alertjq", ".response[]", "JQ filter for alert output")
	clusterAlertsCmd.Flags().StringVar(&clusterAlertsOptions.Filters, "filters", "", "Filters to pass to API")
	clusterAlertsCmd.Flags().Int64Var(&clusterAlertsOptions.StartTime, "stime", startTime, "Start time in epoch format (default: 2 days ago)")
	clusterAlertsCmd.Flags().Int64Var(&clusterAlertsOptions.EndTime, "etime", endTime, "End time in epoch format (default: now)")
	clusterAlertsCmd.Flags().BoolVar(&clusterAlertsOptions.NoPager, "noPager", false, "Dumps complete list")
	clusterAlertsCmd.Flags().BoolVar(&clusterAlertsOptions.JsonFormat, "json", false, "Flag to list alerts in the JSON format")
	clusterAlertsCmd.Flags().IntVar(&clusterAlertsOptions.Page, "page", 0, "Page number for alerts listing")
	clusterAlertsCmd.Flags().IntVar(&clusterAlertsOptions.PageSize, "page-size", 50, "Number of alerts to list per page")
}
