package onboard

import (
	"bytes"
	"crypto/md5" // #nosec G501 only used for calculating existing file hash
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"text/tabwriter"

	"github.com/accuknox/accuknox-cli-v2/pkg/common"
)

const kubearmorBinaryPath = "/opt/kubearmor/kubearmor"

var binNames = map[string]string{
	"kubearmor":                         common.KAconfigPath,
	"kubearmor-vm-adapter":              common.VmAdapterConfigPath,
	"sumengine":                         common.SumEngineConfigPath,
	"kubearmor-relay-server":            common.RelayServerConfigPath,
	"spire-agent":                       common.SpireConfigPath,
	"shared-informer-agent":             common.SIAconfigPath,
	"accuknox-policy-enforcement-agent": common.PEAconfigPath,
	"accuknox-feeder-service":           common.FSconfigPath,
	"hardening":                         common.HardeningAgentConfigPath,
	"discover":                          common.DiscoverConfigPath,
}

type VersionSource string

const (
	SourceMD5     VersionSource = "verified" // matched checksum, exact
	SourceConfig  VersionSource = "config"   // from saved config, may be stale
	SourceUnknown VersionSource = "unknown"
)

type VersionInfo struct {
	Agent  string
	Image  string
	Source VersionSource
	MD5    string
}

type LegacyVersionSchema struct {
	Version string `json:"Version"`
	MD5     string `json:"MD5"`
}

type configJSONObj struct {
	ClusterConfig `json:"cluster_config"`
}

func DetermineAgentVersions() error {

	knoxctlConfigPath := filepath.Clean(filepath.Join(common.SystemdKnoxctlDir, common.KnoxctlConfigFilename))

	imageMap, err := buildImageMap(knoxctlConfigPath)
	if err != nil {
		return err
	}

	manifest := common.GetChecksumsInfo()

	var infos []*VersionInfo
	for binName := range imageMap {
		path := binNames[binName]
		info, ok, err := GetAgentVersion(binName, path, manifest, imageMap)
		if err != nil {
			fmt.Println("failed to get version", binName, err)
			continue
		}
		if !ok {
			continue
		}

		infos = append(infos, info)
	}

	printVersionInfo(infos)

	return nil
}

func printVersionInfo(infos []*VersionInfo) {
	w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)

	fmt.Fprintf(w, "AGENT\tIMAGE\tSTATUS\n")

	var unverified []*VersionInfo

	for _, info := range infos {
		var status string

		switch info.Source {
		case SourceMD5:
			status = "Verified"
		case SourceConfig:
			status = "Unverified (from config)"
			unverified = append(unverified, info)
		default:
			status = "Unverified"
			unverified = append(unverified, info)
		}

		image := info.Image
		if image == "" {
			image = "unknown"
		}

		fmt.Fprintf(w, "%s\t%s\t%s\n", info.Agent, image, status)
	}

	w.Flush()

	if len(unverified) == 0 {
		fmt.Println("\nAll agents are MD5 checksum verified.")
		return
	}

	fmt.Println("\nUnverified Agents — MD5 Checksums:")
	w2 := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
	fmt.Fprintf(w2, "AGENT\tMD5\n")
	for _, info := range unverified {
		md5Val := info.MD5
		if md5Val == "" {
			md5Val = "n/a"
		}
		fmt.Fprintf(w2, "%s\t%s\n", info.Agent, md5Val)
	}
	w2.Flush()

}

func buildImageMap(knoxctlConfigPath string) (map[string]string, error) {
	hunchConfig, err := loadKnoxctlConfig(knoxctlConfigPath)
	if err != nil {
		return nil, err
	}
	if hunchConfig == nil {
		return map[string]string{}, nil // empty map, callers just won't find anything
	}

	if hunchConfig.WorkerNode {
		return map[string]string{
			"kubearmor":            hunchConfig.KubeArmorImage,
			"kubearmor-vm-adapter": hunchConfig.KubeArmorVMAdapterImage,
			"sumengine":            hunchConfig.SumEngineImage,
		}, nil
	}

	return map[string]string{
		"kubearmor":                         hunchConfig.KubeArmorImage,
		"kubearmor-vm-adapter":              hunchConfig.KubeArmorVMAdapterImage,
		"sumengine":                         hunchConfig.SumEngineImage,
		"kubearmor-relay-server":            hunchConfig.KubeArmorRelayServerImage,
		"spire-agent":                       hunchConfig.SPIREAgentImage,
		"shared-informer-agent":             hunchConfig.SIAImage,
		"accuknox-policy-enforcement-agent": hunchConfig.PEAImage,
		"accuknox-feeder-service":           hunchConfig.FeederImage,
		"hardening":                         hunchConfig.HardeningAgentImage,
		"discover":                          hunchConfig.DiscoverImage,
	}, nil
}

func loadKnoxctlConfig(knoxctlConfigPath string) (*configJSONObj, error) {
	knoxctlConfigBytes, err := os.ReadFile(knoxctlConfigPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil // no config file at all — not an error, just absent
		}
		return nil, err
	}

	// Treat empty or whitespace-only content as "no config"
	if len(bytes.TrimSpace(knoxctlConfigBytes)) == 0 {
		return nil, nil
	}

	var hunchConfig configJSONObj
	if err := json.Unmarshal(knoxctlConfigBytes, &hunchConfig); err != nil {
		return nil, fmt.Errorf("parsing knoxctl config: %w", err)
	}

	return &hunchConfig, nil
}

func DetermineKAVersionLegacy() (string, error) {
	kubearmorBinaryPath := filepath.Clean(filepath.Join(common.KAconfigPath, "kubearmor"))
	_, err := os.Stat(kubearmorBinaryPath)
	if err != nil {
		return "", err
	}

	kubearmorBinaryData, err := os.ReadFile(kubearmorBinaryPath)
	if err != nil {
		return "", err
	}

	md5Sum := md5.Sum(kubearmorBinaryData) // #nosec G401 (this is something already present on the system)
	md5SumString := hex.EncodeToString(md5Sum[:16])

	resp, err := http.Get("https://raw.githubusercontent.com/accuknox/pkgversions/refs/heads/main/versions.json")
	if err != nil {
		return "", err
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("got HTTP status %s", resp.Status)
	}

	var legacyVersionObjects []LegacyVersionSchema

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	if err := json.Unmarshal(responseBody, &legacyVersionObjects); err != nil {
		return "", err
	}

	version := "unknown"
	for _, obj := range legacyVersionObjects {
		if obj.MD5 == string(md5SumString) {
			version = obj.Version
		}
	}

	return version, nil
}

func DetermineVersion(binName, configPath string, manifest common.ChecksumManifest) (string, error) {
	binaryPath := filepath.Clean(filepath.Join(configPath, binName))
	_, err := os.Stat(binaryPath)
	if err != nil {
		return "", err
	}

	binaryData, err := os.ReadFile(binaryPath)
	if err != nil {
		return "", err
	}

	md5Sum := md5.Sum(binaryData) // #nosec G401 (this is something already present on the system)
	md5SumString := hex.EncodeToString(md5Sum[:])

	for _, agent := range manifest.Agents {
		for _, v := range agent.Versions {
			if v.MD5 == md5SumString {
				return fmt.Sprintf("%s: %s/%s/%s:%s_%s",
					agent.Label, agent.Registry, agent.Org, agent.Name, v.Version, v.Platform), nil
			}
		}
	}

	return "unknown", nil
}

func GetAgentVersion(binName, configPath string, checksum common.ChecksumManifest, imageMap map[string]string) (*VersionInfo, bool, error) {
	info, err := DetermineVersionFromMD5(binName, configPath, checksum)
	if err != nil {
		return nil, false, err
	}

	// binary wasn't found at all — nothing to report for this agent
	if info.MD5 == "" && info.Source != SourceMD5 {
		return nil, false, nil
	}

	if info.Source == SourceMD5 {
		return info, true, nil
	}

	computedMD5 := info.MD5

	if configInfo, ok := DetermineVersionFromConfig(binName, imageMap); ok {
		configInfo.MD5 = computedMD5
		return configInfo, true, nil
	}

	if computedMD5 == "" {
		return nil, false, nil
	}

	return &VersionInfo{
		Agent:  binName,
		Image:  "unknown",
		Source: SourceUnknown,
		MD5:    computedMD5,
	}, true, nil
}

func DetermineVersionFromMD5(binName, configPath string, manifest common.ChecksumManifest) (*VersionInfo, error) {
	binaryPath := filepath.Clean(filepath.Join(configPath, binName))
	if _, err := os.Stat(binaryPath); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return &VersionInfo{}, nil // binary not present — not an error, just unverifiable
		}
		return nil, err
	}

	binaryData, err := os.ReadFile(binaryPath)
	if err != nil {
		return nil, err
	}

	md5Sum := md5.Sum(binaryData) // #nosec G401
	md5SumString := hex.EncodeToString(md5Sum[:])

	for _, agent := range manifest.Agents {
		for _, v := range agent.Versions {
			if v.MD5 == md5SumString {
				image := fmt.Sprintf("%s/%s/%s:%s_%s", agent.Registry, agent.Org, agent.Name, v.Version, v.Platform)
				return &VersionInfo{
					Agent:  agent.Label,
					Image:  image,
					Source: SourceMD5,
					MD5:    md5SumString,
				}, nil
			}
		}
	}

	return &VersionInfo{
		MD5: md5SumString,
	}, nil // no match, not an error — caller falls back
}

func DetermineVersionFromConfig(agentKey string, imageMap map[string]string) (*VersionInfo, bool) {
	image, ok := imageMap[agentKey]
	if !ok || image == "" {
		return nil, false
	}
	return &VersionInfo{
		Agent:  agentKey,
		Image:  image,
		Source: SourceConfig,
	}, true
}
