package common

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/accuknox/accuknox-cli-v2/pkg/logger"
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
	releaseInfo, err := unmarshal(releaseInfoFile)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	ReleaseInfo = releaseInfo
}

func unmarshal(content []byte) (map[string]ReleaseMetadata, error) {
	releaseInfo := make(map[string]ReleaseMetadata, 0)
	err := json.Unmarshal(content, &releaseInfo)
	return releaseInfo, err
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

func GetLatestReleaseInfoFromEmbedded() (string, ReleaseMetadata) {
	releaseInfo, err := unmarshal(releaseInfoFile)
	if err != nil {
		return "", ReleaseMetadata{}
	}
	latestRelease := "v0.0.0"
	for v := range releaseInfo {
		if semver.Compare(v, latestRelease) > 0 {
			latestRelease = v
		}
	}
	return latestRelease, releaseInfo[latestRelease]
}

func GetReleaseFromBackup(path, version string) (string, ReleaseMetadata) {

	FileName := filepath.Join(filepath.Clean(path), "release.json.bak")

	data, err := os.ReadFile(filepath.Clean(FileName))
	if err != nil {
		return "", ReleaseMetadata{}
	}
	releaseInfo, err := unmarshal(data)
	if err != nil {
		return "", ReleaseMetadata{}
	}
	return version, releaseInfo[version]

}

func GetOrWriteReleaseInfo(releaseFile, path string) (string, error) {

	defaultFileName := filepath.Join(filepath.Clean(path), "release.json")

	fileContent := releaseInfoFile

	if err := createDir(defaultFileName); err != nil {
		return "", err
	}

	if releaseFile != "" {
		data, err := os.ReadFile(filepath.Clean(releaseFile))
		if err != nil {
			return "", err
		}
		releaseInfo, err := unmarshal(data)
		if err != nil {
			return "", err
		}
		ReleaseInfo = releaseInfo
		fileContent = data
		if _, err := os.Stat(defaultFileName); err == nil {
			if err := os.Rename(defaultFileName, defaultFileName+".bak"); err != nil {
				return "", err
			}
		}

		return fmt.Sprintf("Using user defined release file %s", releaseFile), nil

	}
	if _, err := os.Stat(defaultFileName); err == nil {

		data, err := os.ReadFile(filepath.Clean(defaultFileName))
		if err != nil {
			return "", err
		}

		if !bytes.Equal(data, releaseInfoFile) {
			logger.Info1("Old release file found at %s, creating backup at %s", defaultFileName, defaultFileName+".bak")
			if err := os.Rename(defaultFileName, defaultFileName+".bak"); err != nil {
				return "", err
			}
			logger.Info2("To use this release file, use --release-file %v ", defaultFileName+".bak")
		}
	}

	return fmt.Sprintf("Release file written to %s", defaultFileName), os.WriteFile(defaultFileName, fileContent, 0o644) // #nosec G306
}

func createDir(path string) error {
	dir := filepath.Dir(path)
	if dir == "." {
		return nil
	}
	return os.MkdirAll(dir, os.ModeDir|os.ModePerm)
}
