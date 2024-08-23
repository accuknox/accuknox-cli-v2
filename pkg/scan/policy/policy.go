package policy

import (
	"archive/zip"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"sigs.k8s.io/yaml"
)

// GetPolicy is used to fetch and store the policy templates
type GetPolicy struct {
	// URL to download the policy templates
	ZipURL string

	// Policy storage in-mem (slice of policies)
	PolicyCache []*KubeArmorPolicy
}

func NewGenerator(zipURL string) *GetPolicy {
	return &GetPolicy{
		ZipURL:      zipURL,
		PolicyCache: make([]*KubeArmorPolicy, 0),
	}
}

func (gp *GetPolicy) FetchTemplates() error {
	resp, err := http.Get(gp.ZipURL)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	tempZip, err := os.CreateTemp("", "repo-*.zip")
	if err != nil {
		return fmt.Errorf("error creating temp file: %v", err)
	}
	defer os.Remove(tempZip.Name())
	defer tempZip.Close()

	_, err = io.Copy(tempZip, resp.Body)
	if err != nil {
		return fmt.Errorf("error writing zip content: %v", err)
	}

	zipReader, err := zip.OpenReader(tempZip.Name())
	if err != nil {
		return fmt.Errorf("error opening zip: %v", err)
	}
	defer zipReader.Close()

	for _, file := range zipReader.File {
		if isPolicyFile(file.Name) {
			content, err := readZipFile(file)
			if err != nil {
				fmt.Printf("Error reading %s: %v\n", file.Name, err)
				continue
			}

			policy, err := parsePolicy(content)
			if err != nil {
				fmt.Printf("Error parsing policy %s: %v\n", file.Name, err)
				continue
			}

			gp.PolicyCache = append(gp.PolicyCache, policy)
		}
	}

	fmt.Printf("Debug: Total policies fetched: %d\n", len(gp.PolicyCache))

	return nil
}

func isPolicyFile(name string) bool {
	return strings.Contains(name, "hsp") && strings.HasSuffix(name, ".yaml")
}

func readZipFile(file *zip.File) (string, error) {
	fileReader, err := file.Open()
	if err != nil {
		return "", err
	}
	defer fileReader.Close()

	content, err := io.ReadAll(fileReader)
	if err != nil {
		return "", err
	}

	return string(content), nil
}

func parsePolicy(content string) (*KubeArmorPolicy, error) {
	var policy KubeArmorPolicy
	err := yaml.Unmarshal([]byte(content), &policy)
	if err != nil {
		return nil, err
	}

	return &policy, nil
}
