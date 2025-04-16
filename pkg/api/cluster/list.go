package cluster

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/accuknox/accuknox-cli-v2/pkg/api/asset"
	"github.com/accuknox/accuknox-cli-v2/pkg/config"
	"github.com/accuknox/accuknox-cli-v2/pkg/logger"
	"github.com/itchyny/gojq"
	"github.com/olekukonko/tablewriter"
)

const ClusterListDescription = `
1. knoxctl api cluster list --clusterjq '.[] | select(.ClusterName|test("<sub-string>."))' --nodes
... list all the clusters with substring in its names and list all the nodes in those clusters

2. knoxctl api cluster list --clusterjq '.[] | select((.type == "vm") and (.Status == "Inactive"))
... list all the Inactive VM clusters and print their ID,name,status

3. knoxctl api cluster list --nodes --nodejq '.result[] | select(.NodeName == "node-name")'
... list node info for specified 'node-name'

NOTE: In clusterjq flag ".[]" is an array we get from AccuKnox API and then further we can provide a condition, as shown in above example, this condition will be applied on every cluster we get from API. If no further condition is applied, it will dump all the clusters.
In nodejq flag ".[]result" is an array we get from AccuKnox API and then further we can provide a condition, as shown in above example, this condition will be applied on every cluster's node dump we get from API. If no further condition is applied, it will dump all the nodes for that respective cluster.

Node json object example for filtering:
"result": [
      {
        "AlertsCount": 0,
        "ClusterID": 188,
        "ContainerCount": 0,
        "ID": 2788,
        "LabelCount": 5,
        "Location": "",
        "NamespaceCount": 0,
        "NodeName": "simulated-node-d6bc422f",
        "PodCount": 0,
        "PolicyCount": 0,
        "Status": "Inactive",
        "WorkspaceID": 11,
        "created_at": "0001-01-01T00:00:00Z",
        "last_updated_time": "2024-10-14T06:37:49.084513Z",
        "tags": null
      }
    ]

Any of these fileds can be used to filer nodes further

`

type CLusterListOptions struct {
	ClusterListJQ string
	NodeJQ        string
	CWPP_URL      string
	JsonFormat    bool
	NoPager       bool
	ShowNodes     bool
	ClusterName   string
	Token         string
	Tenant_id     string
	CfgFile       string
	Page          int
	PageSize      int
}

type Cluster struct {
	ID          float64 `json:"ID"`
	ClusterName string  `json:"ClusterName"`
	Status      string  `json:"Status"`
}

type nodeInfo struct {
	Total_Record float64 `json:"total_record"`
	Result       []node  `json:"result"`
}
type node struct {
	ID       float64 `json:"ID"`
	NodeName string  `json:"NodeName"`
	Status   string  `json:"Status"`
}

var ClusterInfo Cluster

func FetchClusterInfo(options CLusterListOptions) {
	var clustersData []Cluster
	var table *tablewriter.Table
	var tableNode *tablewriter.Table

	cursorCount := 0
	cursor := [4]string{"|", "/", "—", "\\"}

	if !options.JsonFormat {
		go func() {
			for {
				select {
				case <-done:
					return
				default:
					fmt.Fprintf(os.Stderr, "\rFetching clusters: %s", cursor[cursorCount])
					cursorCount = (cursorCount + 1) % len(cursor)
					time.Sleep(100 * time.Millisecond)
				}
			}
		}()
	}

	clusters := fetchClustersList(config.Cfg.CWPP_URL)

	clusterData, err := jqFilter(clusters, options.ClusterListJQ)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error applying jq filter: %v\n", err)
		return
	}
	if len(clusterData) == 0 {
		fmt.Println("No clusters found matching the provided criteria.")
		return
	}

	for _, cluster := range clusterData {
		bytes, err := json.Marshal(cluster)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error marshaling: %v\n", err)
			return
		}

		if err := json.Unmarshal(bytes, &ClusterInfo); err != nil {
			fmt.Fprintf(os.Stderr, "Error unmarshaling: %v\n", err)
			return
		}
		clustersData = append(clustersData, ClusterInfo)
	}

	if options.ClusterName == "" && !(options.ShowNodes && options.JsonFormat) {
		if !options.JsonFormat {
			logger.Print("\nTotal clusters found: %v", len(clusterData))
			table = tablewriter.NewWriter(os.Stdout)
			table.SetHeader([]string{"ID", "Cluster-Name", "Status"})
		}

		for _, cluster := range clustersData {
			if !options.JsonFormat {
				table.Append([]string{strconv.FormatFloat(cluster.ID, 'f', -1, 64), cluster.ClusterName, cluster.Status})
				table.SetRowLine(true)
			}
		}

		if !options.JsonFormat {
			done <- true
			table.Render()
		}
		if options.JsonFormat {
			asset.PrintJSON(clusterData, options.NoPager, options.JsonFormat)
		}
	}

	var clusterNodes []interface{}
	if options.ShowNodes {
		if !options.JsonFormat {
			tableNode = tablewriter.NewWriter(os.Stdout)
			tableNode.SetHeader([]string{"Node-ID", "Cluster-Name", "Node-Name", "Status"})
		}
		for _, cluster := range clustersData {
			node := fetchNodes(cluster, options, tableNode)
			if node == nil {
				continue
			}
			clusterNodes = append(clusterNodes, node)
		}

		if !options.JsonFormat {
			logger.Print("\nNode Information : ")
			tableNode.Render()
		} else {
			asset.PrintJSON(clusterNodes, true, options.JsonFormat)
		}
	}
}

// fetchClustersList fetches the list of onboarded clusters from the API
func fetchClustersList(url string) interface{} {
	apiURL := fmt.Sprintf("%s/cluster-onboarding/api/v1/get-onboarded-clusters?wsid=%s", url, config.Cfg.TENANT_ID)

	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating HTTP request: %v\n", err)
		return nil
	}
	req.Header.Set("Authorization", "Bearer "+config.Cfg.TOKEN)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Tenant-ID", config.Cfg.TENANT_ID)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error making API call: %v\n", err)
		return nil
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading response body: %v\n", err)
		return nil
	}

	var jsonResponse interface{}
	err = json.Unmarshal(body, &jsonResponse)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error unmarshalling JSON: %v\n", err)
		logger.Error("api responce: %v\n", string(body))
		return nil
	}

	return jsonResponse
}

// fetchNodes fetches the nodes for a given cluster
func fetchNodes(ClusterInfo Cluster, options CLusterListOptions, tableNode *tablewriter.Table) map[string]interface{} {
	if options.ClusterName != "" && options.ClusterName != ClusterInfo.ClusterName {
		return nil
	}

	apiURL := fmt.Sprintf("%s/cm/api/v1/cluster-management/nodes-in-cluster", config.Cfg.CWPP_URL)

	var clusterNode []interface{}
	polPerPage := options.PageSize // Number of policies per page
	pagePrevious := 0
	for i := 1; ; i++ {
		if options.Page != 0 && i > options.Page {
			break
		}
		pageNext := pagePrevious + polPerPage

		requestPayload := map[string]interface{}{
			"workspace_id":  config.Cfg.TENANT_ID,
			"cluster_id":    []interface{}{ClusterInfo.ID},
			"from_time":     []int64{},
			"to_time":       []int64{},
			"page_previous": pagePrevious,
			"page_next":     pageNext,
		}

		payloadBytes, err := json.Marshal(requestPayload)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error preparing request payload: %v\n", err)
			return nil
		}

		req, err := http.NewRequest("POST", apiURL, bytes.NewBuffer(payloadBytes))
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error creating HTTP request: %v\n", err)
			return nil
		}
		req.Header.Set("Authorization", "Bearer "+config.Cfg.TOKEN)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Tenant-ID", config.Cfg.TENANT_ID)

		client := &http.Client{}
		resp, err := client.Do(req)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error fetching nodes: %v\n", err)
			return nil
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			fmt.Fprintf(os.Stderr, "Error: Received status code %d with body: %s\n", resp.StatusCode, body)
			return nil
		}

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error reading response: %v\n", err)
			return nil
		}

		var nodeData map[string]interface{}
		err = json.Unmarshal(body, &nodeData)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error unmarshalling nodes response: %v\n", err)
			logger.Error("api responce: %v\n", string(body))
			return nil
		}

		// jq filtering
		nodes, err := jqFilter(nodeData, options.NodeJQ)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error applying jq filter: %v\n", err)
			return nil
		}

		total_record, err := jqFilter(nodeData, ".total_record")
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error applying jq filter: %v\n", err)
			return nil
		}

		var record float64
		if len(total_record) > 0 {
			record = total_record[0].(float64)
		}

		for _, nodeData := range nodes {
			var nodeInfo node
			nodeBytes, _ := json.MarshalIndent(nodeData, "", "  ")
			if !options.JsonFormat {
				err = json.Unmarshal(nodeBytes, &nodeInfo)
				if err != nil {
					fmt.Fprintf(os.Stderr, "Error unmarshalling node: %v\n", err)
					return nil
				}

				fmt.Printf("\rTotal nodes found : %v, fetching nodes from: %d to next: %d for cluster %s", record, pagePrevious, pageNext, ClusterInfo.ClusterName)

				tableNode.Append([]string{strconv.FormatFloat(nodeInfo.ID, 'f', -1, 64), ClusterInfo.ClusterName, nodeInfo.NodeName, nodeInfo.Status})
				tableNode.SetRowLine(true)
			}
		}

		if len(nodes) == 0 && record <= float64(pageNext) {
			break
		}

		if options.JsonFormat {
			clusterNode = append(clusterNode, nodes...)
		}
		pagePrevious = pageNext
	}
	if options.JsonFormat && clusterNode != nil {
		clusterNodes := map[string]interface{}{
			"cluster": ClusterInfo.ClusterName,
			"result":  clusterNode,
		}
		return clusterNodes
	}
	return nil
}

func jqFilter(data interface{}, jqFilter string) ([]interface{}, error) {
	query, err := gojq.Parse(jqFilter)
	if err != nil {
		return nil, fmt.Errorf("invalid jq filter: %v", err)
	}

	iter := query.Run(data)
	var results []interface{}
	for {
		v, ok := iter.Next()
		if !ok {
			break
		}
		if err, ok := v.(error); ok {
			return nil, fmt.Errorf("jq error: %v", err)
		}
		results = append(results, v)
	}
	return results, nil
}
