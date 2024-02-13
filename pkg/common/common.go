package common

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"

	"github.com/accuknox/dev2/api/grpc/v2/summary"
	"github.com/kubearmor/kubearmor-client/k8s"
	"github.com/kubearmor/kubearmor-client/utils"
	"github.com/olekukonko/tablewriter"
)

func ConnectGrpc(c *k8s.Client, grpc string) (string, error) {
	if grpc != "" {
		return grpc, nil
	} else {
		if val, ok := os.LookupEnv("DISCOVERY_SERVICE"); ok {
			grpc = val
		} else {
			pf, err := utils.InitiatePortForward(c, Port, Port, MatchLabels, ServiceName)
			if err != nil {
				return "", err
			}
			grpc = "localhost:" + strconv.FormatInt(pf.LocalPort, 10)
		}
		return grpc, nil
	}
}

func DisplayOutput(events *summary.Events, revDNSLookup bool, cluster, namespace, workloadType, workloadName, outputType string) {

	if len(events.Process) <= 0 && len(events.File) <= 0 && len(events.Ingress) <= 0 && len(events.Egress) <= 0 {
		return
	}

	writeWorkloadInfoToTable(cluster, namespace, workloadType, workloadName)

	// Colored Status for Allow and Deny
	//agc := ansi.ColorFunc("green")
	//arc := ansi.ColorFunc("red")
	//ayc := ansi.ColorFunc("yellow")

	if len(events.Process) > 0 {
		fmt.Printf("\nProcess Data\n")
		//WriteTable(SysProcHeader, procRowData)
		//tableOutput(SysProcHeader, procRowData, "Process Data\n")
		WriteTable(SysProcHeader, rowDataProcessFile(events.Process))
		fmt.Printf("\n")
	} else if len(events.File) > 0 {
		fmt.Printf("\nFile Data\n")
		//WriteTable(SysProcHeader, procRowData)
		//tableOutput(SysFileHeader, procRowData, "File Data\n")
		WriteTable(SysFileHeader, rowDataProcessFile(events.File))
		fmt.Printf("\n")
	} else if len(events.Ingress) > 0 {
		fmt.Printf("\nIngress Data\n")
		//WriteTable(SysNwHeader, inNwRowData)
		//tableOutput(SysNwHeader, rowNetworkData(events.Ingress), "Ingress Connections\n")
		WriteTable(SysNwHeader, rowNetworkData(events.Ingress))
		fmt.Printf("\n")
	} else if len(events.Egress) > 0 {
		fmt.Printf("\nEgress Data\n")
		//WriteTable(SysNwHeader, inNwRowData)
		//tableOutput(SysNwHeader, rowNetworkData(events.Egress), "Ingress Connections\n")
		WriteTable(SysNwHeader, rowNetworkData(events.Egress))
		fmt.Printf("\n")
	} else if len(events.Bind) > 0 {
		fmt.Printf("\nBind Data\n")
		//WriteTable(SysNwHeader, inNwRowData)
		//tableOutput(SysBindNwHeader, rowNetworkData(events.Bind), "Ingress Connections\n")
		WriteTable(SysBindNwHeader, rowNetworkData(events.Bind))
		fmt.Printf("\n")
	}

}

func rowDataProcessFile(events []*summary.ProcessFileEvent) [][]string {
	var rowData [][]string
	for _, data := range events {
		strSlice := []string{}
		strSlice = append(strSlice, data.Source)
		strSlice = append(strSlice, data.Destination)
		strSlice = append(strSlice, strconv.Itoa(int(data.Count)))
		strSlice = append(strSlice, strconv.Itoa(int(data.UpdatedTime)))
		//if procData.action == "Allow" {
		//	procStrSlice = append(procStrSlice, agc(procData.Status))
		//} else if procData.action == "Audit" {
		//	procStrSlice = append(procStrSlice, ayc(procData.Status))
		//} else {
		//	procStrSlice = append(procStrSlice, arc(procData.Status))
		//}
		rowData = append(rowData, strSlice)
	}
	sort.Slice(rowData[:], func(i, j int) bool {
		for x := range rowData[i] {
			if rowData[i][x] == rowData[j][x] {
				continue
			}
			return rowData[i][x] < rowData[j][x]
		}
		return false
	})
	return rowData
}

func rowNetworkData(events []*summary.NetworkEvent) [][]string {
	var rowData [][]string
	for _, data := range events {
		nwStrSlice := []string{}
		//domainName := dnsLookup(data.IP, revDNSLookup)
		nwStrSlice = append(nwStrSlice, data.Protocol)
		nwStrSlice = append(nwStrSlice, data.Command)
		nwStrSlice = append(nwStrSlice, data.Ip)
		nwStrSlice = append(nwStrSlice, strconv.Itoa(int(data.Port)))
		//nwStrSlice = append(nwStrSlice, data.Namespace)
		//nwStrSlice = append(nwStrSlice, data.Labels)
		nwStrSlice = append(nwStrSlice, strconv.Itoa(int(data.Count)))
		nwStrSlice = append(nwStrSlice, strconv.Itoa(int(data.UpdatedTime)))
		rowData = append(rowData, nwStrSlice)
	}
	return rowData
}

func writeWorkloadInfoToTable(cluster, namespace, workloadType, workloadName string) {

	fmt.Printf("\n")
	workloadInfo := [][]string{
		{"Cluster", cluster},
		{"Namespace", namespace},
		{"Workload type", workloadType},
		{"Workload name", workloadName},
	}
	table := tablewriter.NewWriter(os.Stdout)
	table.SetBorder(false)
	table.SetTablePadding("\t")
	table.SetCenterSeparator("")
	table.SetColumnSeparator("")
	table.SetRowSeparator("")
	table.SetAlignment(tablewriter.ALIGN_LEFT)
	for _, v := range workloadInfo {
		table.Append(v)
	}
	table.Render()
}

// GetDefaultConfigPath returns home dir along with an error
func GetDefaultConfigPath() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	if homeDir == "" {
		return "", fmt.Errorf("Home directory not found")
	}

	configPath := filepath.Join(homeDir, DefaultConfigPathDirName)

	return configPath, nil
}
