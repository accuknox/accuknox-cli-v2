package cluster

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/accuknox/accuknox-cli-v2/pkg/api/asset"
	"github.com/accuknox/accuknox-cli-v2/pkg/config"
	"github.com/accuknox/accuknox-cli-v2/pkg/logger"
)

const ClusterAlertDescription = `
1. knoxctl api cluster alerts --alertjq '.response[] | select(.Resource // "unknown" | test("ziggy"))' 
... list all the alerts containing 'ziggy' in the Resource filter

2. knoxctl api cluster alerts --filters '{"field":"HostName","value":"store54055","op":"match"}' --alertjq '.response[] | "hostname=\(.HostName),resource=\(.Resource//""),UID=\(.UID),operation=\(.Operation)"'
... get all alerts for HostName="store54055" and print the response in following csv format hostname,resource,UID,operation

NOTE: --filters are passed directly to the AccuKnox API. --alertjq operates on the output of the AccuKnox API response. It is recommended to use --filters as far as possible. However, you can use regex/jq based matching criteria with --alertjq.
In alertjq flag ".response[]" is an array we get from AccuKnox API response and then further we can provide a condition(on top of the array we get), as shown in above example, this condition will be applied on every alert we get and will list only which statisfy it. If no further condition is applied, it will dump all the alerts.

`

type ClusterALertOptions struct {
	ClusterAlertJQ string
	AlertJQ        string
	CWPP_URL       string
	JsonFormat     bool
	NoPager        bool
	ShowNodes      bool
	Filters        string
	StartTime      int64
	EndTime        int64
	AlertType      string
	Page           int
	PageSize       int
	Token          string
	Tenant_id      string
	CfgFile        string
	LogType        string
	Cluster_id     string
}

var done = make(chan bool, 1)

type FilterField struct {
	Field string `json:"field"`
	Value string `json:"value"`
	Op    string `json:"op"`
}

func FetchClusterAlerts(options ClusterALertOptions) {
	cursorCount := 0
	cursor := [4]string{"|", "/", "—", "\\"}

	if !options.JsonFormat {
		go func() {
			for {
				select {
				case <-done:
					return
				default:
					fmt.Fprintf(os.Stderr, "\rFetching Alerts: %s", cursor[cursorCount])
					cursorCount = (cursorCount + 1) % len(cursor)
					time.Sleep(100 * time.Millisecond)
				}
			}
		}()
	}
	clusters := fetchClustersList(config.Cfg.CWPP_URL)
	clusterData, _ := jqFilter(clusters, options.ClusterAlertJQ)
	if len(clusterData) == 0 {
		fmt.Println("No clusters found matching the provided criteria.")
		return
	}

	var clusterIDs []float64
	for _, cluster := range clusterData {
		bytes, err := json.Marshal(cluster)
		if err != nil {
			logger.Error("error while marshaling: %v", err)
			return
		}

		if err := json.Unmarshal(bytes, &ClusterInfo); err != nil {
			logger.Error("error while unmarshaling: %v", err)
			return
		}
		clusterIDs = append(clusterIDs, ClusterInfo.ID)
	}

	queryClusterAlerts(clusterIDs, options)
}

func queryClusterAlerts(clusterIDs []float64, options ClusterALertOptions) {
	var allResults []interface{}
	clusterIDsS := []string{}

	apiURL := fmt.Sprintf("%s/monitors/v1/alerts/events?orderby=desc", config.Cfg.CWPP_URL)

	if options.Cluster_id != "" {
		clusterIDsS = strings.Split(options.Cluster_id, ",")
	}

	for pageID := 1; ; pageID++ {
		if options.Page != 0 && pageID > options.Page {
			if !options.JsonFormat {
				done <- true
			}
			break
		}

		var filter FilterField
		var filters []FilterField
		if options.Filters != "" {
			err := json.Unmarshal([]byte(options.Filters), &filter)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Invalid filters format: %v\n", err)
				return
			}
		}

		filters = append(filters, filter)

		requestPayload := map[string]interface{}{
			"FromTime":    options.StartTime,
			"ToTime":      options.EndTime,
			"PageId":      pageID,
			"PageSize":    options.PageSize,
			"Filters":     filters,
			"ClusterID":   clusterIDsS,
			"View":        "List",
			"Type":        options.AlertType,
			"WorkspaceID": config.Cfg.TENANT_ID,
			"LogType":     options.LogType,
		}

		if options.Filters == "" {
			requestPayload["Filters"] = []interface{}{}
		}

		payloadBytes, err := json.Marshal(requestPayload)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error preparing request payload: %v\n", err)
			return
		}

		req, err := http.NewRequest("POST", apiURL, bytes.NewBuffer(payloadBytes))
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error creating HTTP request: %v\n", err)
			return
		}
		req.Header.Set("Authorization", "Bearer "+config.Cfg.TOKEN)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Tenant-ID", config.Cfg.TENANT_ID)

		client := &http.Client{}
		resp, err := client.Do(req)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error fetching nodes: %v\n", err)
			return
		}
		defer resp.Body.Close()

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error reading response: %v\n", err)
			return
		}

		var jsonResponse interface{}
		err = json.Unmarshal(body, &jsonResponse)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error unmarshalling JSON: %v\n", err)
			logger.Error("api responce: %v\n", string(body))
			return
		}
		jsonMap, ok := jsonResponse.(map[string]interface{})
		if !ok || jsonMap["response"] == nil {
			if !options.JsonFormat {
				done <- true
			}
			break
		}
		results, err := jqFilter(jsonResponse, options.AlertJQ)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error while applying jq filter: %v\n", err)
			return
		}
		allResults = append(allResults, results...)
	}
	if !options.JsonFormat {
		logger.Print("\nTotal alerts found: %v", len(allResults))
	}
	asset.PrintJSON(allResults, options.NoPager, options.JsonFormat)
}
