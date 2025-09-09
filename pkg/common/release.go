package common

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"golang.org/x/mod/semver"
)

type ReleaseMetadata struct {
	CreationTime          string `json:"creation_time"`
	KubeArmorTag          string `json:"kubearmor_tag"`
	KubeArmorRelayTag     string `json:"kubearmor_relay_tag"`
	KubeArmorVMAdapterTag string `json:"kubearmor_vm_adapter_tag"`
	SPIREAgentImageTag    string `json:"spire_agent_tag"`
	SIATag                string `json:"sia_tag"`
	SIAImage              string `json:"sia_image"`
	PEATag                string `json:"pea_tag"`
	PEAImage              string `json:"pea_image"`
	FeederServiceTag      string `json:"feeder_service_tag"`
	FeederServiceImage    string `json:"feeder_service_image"`
	DiscoverTag           string `json:"discover_tag"`
	DiscoverImage         string `json:"discover_image"`
	SumEngineTag          string `json:"sumengine_tag"`
	SumEngineImage        string `json:"sumengine_image"`
	HardeningAgentTag     string `json:"hardening_agent_tag"`
	HardeningAgentImage   string `json:"hardening_agent_image"`
	RraTag                string `json:"rra_tag"`
	RraImage              string `json:"rra_image"`
}

var (
	//go:embed release.json
	releaseInfoFile []byte

	ReleaseInfo = make(map[string]ReleaseMetadata, 0)
)

func init() {
	err := unmarshal(releaseInfoFile)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func unmarshal(content []byte) error {
	return json.Unmarshal(content, &ReleaseInfo)
}

// returns the latest release according to version tag and not according to
// creation time
// doesn't make much sense because our release strategy on agents-chart
// repo is haywire... but who cares
func GetLatestReleaseInfo() (string, ReleaseMetadata) {
	latestRelease := "v0.0.0"
	for v := range ReleaseInfo {
		if semver.Compare(v, latestRelease) > 0 {
			latestRelease = v
		}
	}

	return latestRelease, ReleaseInfo[latestRelease]
}

func GetOrWriteReleaseInfo(path string) (string, error) {

	cleanFilePath := filepath.Clean(path)
	filePath := filepath.Join(cleanFilePath, "release.json")
	if _, err := os.Stat(filePath); err == nil {
		data, err := os.ReadFile(filePath)
		if err != nil {
			return "", err
		}
		if err := unmarshal(data); err != nil {
			return "", err
		}
		return fmt.Sprintf("Release file found at %s", filePath), nil
	}

	if err := os.MkdirAll(cleanFilePath, os.ModeDir|os.ModePerm); err != nil {
		return "", err
	}

	return fmt.Sprintf("Release file written to %s", filePath), os.WriteFile(filePath, releaseInfoFile, 0o644) // #nosec G306
}
