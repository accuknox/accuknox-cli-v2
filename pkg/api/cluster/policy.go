package cluster

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/accuknox/accuknox-cli-v2/pkg/api/asset"
	"github.com/accuknox/accuknox-cli-v2/pkg/config"
	"github.com/accuknox/accuknox-cli-v2/pkg/logger"
	"github.com/olekukonko/tablewriter"
)

const ClusterPolicyDescription = `
1. knoxctl api cluster policy --clusterjq '.[] | select(.ClusterName|test("gke"))' --policyjq '.list_of_policies[] | select(.name|test("crypto"))'
... get all the policies have 'crypto' in their name for all the clusters having 'gke' in their name

2. knoxctl api cluster policy --clusterjq '.[] | select(.ClusterName|test("gke"))' --policyjq '.list_of_policies[] | select(.namespace_name // "notpresent"|test("agents"))'
... get all the policies in namespace agents ... if no namespace is present then "notpresent" is substituted.

NOTE: In policyjq flag ".list_of_policies[]" is an array we get from AccuKnox API response and then further we can provide a condition(on top of the array we get), as shown in above example, this condition will be applied on every policy we get and will list only which statisfy it. If no further condition is applied, it will dump all the policies.
Here, clusterjq flag has same behaviour as of in "api cluster list" command flag.
`

type ClusterPolicyOptions struct {
	ClusterListJQ string
	PolicyJQ      string
	CWPP_URL      string
	JsonFormat    bool
	NoPager       bool
	Operation     string
	ClusterName   string
	Token         string
	Tenant_id     string
	CfgFile       string
}

type Policy struct {
	ID float64 `json:"policy_id"`
}

var PolicyInfo Policy
var clusterPolicies []map[string]interface{}
var polout string = "policydump"

// fetchPolicies retrieves and processes policies for a given clusterID with pagination
func fetchPolicies(clusterID string, options ClusterPolicyOptions) ([]map[string]interface{}, error) {
	polPerPage := 50 // Number of policies per page
	pagePrevious := 0
	client := &http.Client{}
	table := tablewriter.NewWriter(os.Stdout)
	for {
		pageNext := pagePrevious + polPerPage
		id, _ := strconv.ParseFloat(clusterID, 64)

		requestPayload := map[string]interface{}{
			"workspace_id":  config.Cfg.TENANT_ID,
			"workload":      "k8s",
			"page_previous": pagePrevious,
			"page_next":     pageNext,
			"filter": map[string]interface{}{
				"cluster_id":   []interface{}{id},
				"namespace_id": nil,
				"workload_id":  nil,
				"kind":         nil,
				"node_id":      nil,
				"pod_id":       nil,
				"type":         nil,
				"status":       nil,
				"tags":         nil,
				"name": map[string]interface{}{
					"regex": nil,
				},
				"tldr": map[string]interface{}{
					"regex": nil,
				},
			},
		}

		payloadBytes, err := json.Marshal(requestPayload)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error preparing request payload: %v\n", err)
			return nil, err
		}

		url := fmt.Sprintf("%s/policymanagement/v2/list-policy", config.Cfg.CWPP_URL)
		req, err := http.NewRequest("POST", url, bytes.NewBuffer(payloadBytes))
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error creating HTTP request: %v\n", err)
			return nil, err
		}

		req.Header.Set("Authorization", "Bearer "+config.Cfg.TOKEN)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Tenant-ID", config.Cfg.TENANT_ID)

		// #nosec G704 -- request controlled internally
		resp, err := client.Do(req)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error making API request: %v\n", err)
			return nil, err
		}
		defer resp.Body.Close()

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error reading response body: %v\n", err)
			return nil, err
		}

		var jsonResponse interface{}
		if err := json.Unmarshal(body, &jsonResponse); err != nil {
			fmt.Fprintf(os.Stderr, "Error unmarshaling response: %v\n", err)
			return nil, err
		}
		results, err := jqFilter(jsonResponse, options.PolicyJQ)
		if err != nil {
			if strings.Contains(err.Error(), "null") {
				if pageNext != polPerPage && !options.JsonFormat {
					table.Render()
				} else if !options.JsonFormat {
					fmt.Println("No policies available...")
				}
				return clusterPolicies, nil
			}
			fmt.Fprintf(os.Stderr, "Error jq filtering: %v\n", err)
			return nil, err
		}

		for _, policy := range results {
			policyMap, ok := policy.(map[string]interface{})
			if !ok {
				continue
			}

			var policyID float64
			var name string
			var namespace, category, status, cluster string
			if policyID, ok = policyMap["policy_id"].(float64); !ok {
				logger.Debug("policy_id not found")
			}
			if name, ok = policyMap["name"].(string); !ok {
				logger.Debug("name not found")
			}
			if namespace, ok = policyMap["namespace_name"].(string); !ok {
				namespace = ""
			}
			if category, ok = policyMap["category"].(string); !ok {
				category = ""
			}
			if status, ok = policyMap["status"].(string); !ok {
				status = ""
			}
			if cluster, ok = policyMap["cluster_name"].(string); !ok {
				cluster = ""
			}

			var labelStrings []string

			if rawLabels, ok := policyMap["labels"].([]interface{}); ok {
				for _, item := range rawLabels {
					if labelMap, ok := item.(map[string]interface{}); ok {
						name, nameOk := labelMap["name"].(string)
						value, valueOk := labelMap["value"].(string)
						if nameOk && valueOk {
							labelStrings = append(labelStrings, fmt.Sprintf("%s:%s", name, value))
						}
					}
				}
			}

			var clusterPolicy map[string]interface{}
			if options.JsonFormat {
				clusterPolicy = map[string]interface{}{
					"name":      name,
					"namespace": namespace,
					"category":  category,
					"status":    status,
					"cluster":   cluster,
					"labels":    labelStrings,
				}
			}

			clusterPolicies = append(clusterPolicies, clusterPolicy)
			if !options.JsonFormat {
				table.SetHeader([]string{"Name", "Category", "Status", "Cluster", "Namespace", "Labels"})
				table.Append([]string{name, category, status, cluster, namespace, strings.Join(labelStrings, ", ")})
				table.SetRowLine(true)
			}

			if options.Operation == "dump" {
				fetchPolicy(options, strconv.FormatFloat(policyID, 'f', -1, 64), name, namespace)
			}
		}
		pagePrevious = pageNext
	}
}

// FetchAndProcessPolicies fetches and processes policies for all clusters
func FetchAndProcessPolicies(options ClusterPolicyOptions) {
	var policies []map[string]interface{}

	clusters := fetchClustersList(config.Cfg.CWPP_URL)

	clusterData, err := asset.ApplyJQFilter(clusters, options.ClusterListJQ)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error while applying jq filter: %v\n", err)
		return
	}
	if len(clusterData) == 0 {
		fmt.Println("No clusters found matching the provided criteria.")
		return
	}

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
		if options.ClusterName != "" && ClusterInfo.ClusterName != options.ClusterName {
			continue
		}
		if options.Operation == "dump" {
			cpath := filepath.Join(polout, ClusterInfo.ClusterName)
			err := os.MkdirAll(cpath, 0750)
			if err != nil {
				logger.Error("err: %v\n", err)
			}
		}

		if !options.JsonFormat {
			logger.Print("Fetching policies for cluster: %s", ClusterInfo.ClusterName)
		}
		if options.JsonFormat {
			clusterPolcies, err := fetchPolicies(strconv.FormatFloat(ClusterInfo.ID, 'f', -1, 64), options)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error fetching policies for cluster %s: %v\n", ClusterInfo.ClusterName, err)
			}
			policies = append(policies, clusterPolcies...)
		} else {
			_, err := fetchPolicies(strconv.FormatFloat(ClusterInfo.ID, 'f', -1, 64), options)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error fetching policies for cluster %s: %v\n", ClusterInfo.ClusterName, err)
			}
		}

	}
	if options.JsonFormat {
		jsonPolicies, _ := json.Marshal(policies)
		fmt.Println(string(jsonPolicies))
	}
}

func dumpPolicy(name, namespace, policy string) error {
	filePath := filepath.Join(polout, ClusterInfo.ClusterName, namespace, fmt.Sprintf("%s.yaml", name))
	if err := os.MkdirAll(filepath.Dir(filePath), 0750); err != nil {
		return fmt.Errorf("err: %v", err)
	}

	if err := ioutil.WriteFile(filePath, []byte(policy), 0600); err != nil {
		return fmt.Errorf("failed to write file: %v", err)
	}

	fmt.Printf("Policy dumped successfully: %s\n", filePath)
	return nil
}

func fetchPolicy(options ClusterPolicyOptions, policyID, name, namespace string) {
	apiURL := fmt.Sprintf("%s/policymanagement/v2/policy/%s", config.Cfg.CWPP_URL, policyID)

	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating HTTP request: %v\n", err)
		return
	}
	req.Header.Set("Authorization", "Bearer "+config.Cfg.TOKEN)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Tenant-ID", config.Cfg.TENANT_ID)

	client := &http.Client{}
	// #nosec G704 -- request controlled internally
	resp, err := client.Do(req)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error making API call: %v\n", err)
		return
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading response body: %v\n", err)
		return
	}

	var jsonResponse map[string]interface{}
	err = json.Unmarshal(body, &jsonResponse)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error unmarshalling JSON: %v\n", err)
		logger.Error("api responce: %v\n", string(body))
		return
	}

	yamlData, ok := jsonResponse["yaml"].(string)
	if !ok {
		fmt.Fprintln(os.Stderr, "YAML field not found in response")
		return
	}

	if options.Operation == "dump" {
		err := dumpPolicy(name, namespace, yamlData)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return
		}
	}
}
