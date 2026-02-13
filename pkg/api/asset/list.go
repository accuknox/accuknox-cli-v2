package asset

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/accuknox/accuknox-cli-v2/pkg/config"
	"github.com/accuknox/accuknox-cli-v2/pkg/logger"
	"github.com/itchyny/gojq"
)

const AssetDescription = `
1. knoxctl api asset list
... list all the assets
	
2. knoxctl api asset list --filter "asset_category=Container" --assetjq ".results[] | select(.vulnerabilities.Critical >= 3)"
... list all the assets of category Container and having critical vulnerabilities more or equal to 3

# Asset Categories list
1. Container
2. Storage

NOTE: --filters are passed directly to the AccuKnox API. --assetjq operates on the output of the AccuKnox API response. It is recommended to use --filters as far as possible. However, you can use regex/jq based matching criteria with --assetjq.
In assetjq flag ".results[]" is an array we get from AccuKnox API response and then further we can provide a condition(on top of the array we get), as shown in above example, this condition will be applied on every asset we get and will list only which statisfy it. If no further condition is applied, it will dump all the assets.
`

type Options struct {
	Filter     string
	AssetJQ    string
	CSPM_URL   string
	Token      string
	Tenant_id  string
	CfgFile    string
	Page       int
	PageSize   int
	Timeout    int
	JsonFormat bool
	NoPager    bool
}

func ListAssets(o Options) {
	var allResults []interface{}
	cursorCount := 0
	stime := time.Now()
	otime := stime.Add(time.Duration(o.Timeout) * time.Second)
	apiURL := config.Cfg.CSPM_URL + "/api/v1/assets"
	currentPage := 1
	hasMore := true
	cursor := [4]string{"|", "/", "—", "\\"}

	done := make(chan bool)

	if !o.JsonFormat {
		go func() {
			for {
				select {
				case <-done:
					return
				default:
					fmt.Printf("\rFetching Assets: %s", cursor[cursorCount])
					cursorCount = (cursorCount + 1) % len(cursor)
					time.Sleep(100 * time.Millisecond)
				}
			}
		}()
	}

	for hasMore {
		query := fmt.Sprintf("?page=%d&page_size=%d", currentPage, o.PageSize)
		if o.Filter != "" {
			query += "&" + strings.TrimSpace(o.Filter)
		}

		req, err := http.NewRequest("GET", apiURL+query, nil)
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

		// jq filtering
		results, err := ApplyJQFilter(jsonResponse, o.AssetJQ)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error applying jq filter: %v\n", err)
		}

		allResults = append(allResults, results...)

		hasMore = len(results) > 0 && o.PageSize != 0
		currentPage++
		if o.Page != 0 && currentPage > o.Page {
			break
		}
		if !otime.After(time.Now()) {
			if !o.JsonFormat {
				fmt.Printf("\rRequest cancelled due to Time-Out!\n")
			}
			break
		}
	}
	if !o.JsonFormat {
		done <- true
	}
	if !o.JsonFormat {
		logger.Print("\nTotal assets found: %v", len(allResults))
	}
	PrintJSON(allResults, o.NoPager, o.JsonFormat)
}

func ApplyJQFilter(data interface{}, jqFilter string) ([]interface{}, error) {
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
		m, _ := v.(map[string]interface{})
		if m == nil {
			return nil, nil
		}
		if err, ok := v.(error); ok {
			return nil, fmt.Errorf("jq error: %v", err)
		}
		results = append(results, v)
	}
	return results, nil
}

func PrintJSON(results []interface{}, noPager, jsonFormat bool) {
	if jsonFormat {
		output, _ := json.Marshal(results)
		fmt.Println(string(output))
	} else {
		if len(results) == 0 {
			fmt.Println("No records found")
			return
		}
		output, err := json.MarshalIndent(results, "", "  ")
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error formatting results: %v\n", err)
			return
		}

		if !noPager {
			cmd := exec.Command("less")
			cmd.Stdin = strings.NewReader(string(output))
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			if err := cmd.Run(); err != nil {
				logger.Error("err: %v", err)
			}
		} else {
			fmt.Fprintln(os.Stdout, string(output))
		}
	}

}
