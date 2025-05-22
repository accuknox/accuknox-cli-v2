package onboard

import (
	"crypto/md5" // #nosec G501 only used for calculating existing file hash
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"

	"github.com/accuknox/accuknox-cli-v2/pkg/common"
)

const kubearmorBinaryPath = "/opt/kubearmor/kubearmor"

type LegacyVersionSchema struct {
	Version string `json:"Version"`
	MD5     string `json:"MD5"`
}

func DetermineAgentVersions() (map[string]string, error) {
	configFileNotExist := false
	knoxctlConfigPath := filepath.Clean(filepath.Join(common.SystemdKnoxctlDir, common.KnoxctlConfigFilename))
	_, err := os.Stat(knoxctlConfigPath)
	if err != nil && os.IsNotExist(err) {
		configFileNotExist = true
	} else if err != nil {
		return nil, err
	}

	if configFileNotExist {
		kaVersion, err := DetermineKAVersionLegacy()
		if err != nil {
			return nil, err
		}

		return map[string]string{
			"kubearmor": kaVersion,
		}, nil
	}

	knoxctlConfigBytes, err := os.ReadFile(knoxctlConfigPath)
	if err != nil {
		return nil, err
	}

	type configJSONObj struct {
		ClusterConfig `json:"cluster_config"`
	}
	var hunchConfig configJSONObj

	err = json.Unmarshal(knoxctlConfigBytes, &hunchConfig)
	if err != nil {
		return nil, err
	}

	if hunchConfig.WorkerNode {
		return map[string]string{
			"kubearmor":            hunchConfig.KubeArmorImage,
			"kubearmor-vm-adapter": hunchConfig.KubeArmorVMAdapterImage,
			"summary-engine":       hunchConfig.SumEngineImage,
		}, nil
	} else {
		return map[string]string{
			"kubearmor":                hunchConfig.KubeArmorImage,
			"kubearmor-vm-adapter":     hunchConfig.KubeArmorVMAdapterImage,
			"summary-engine":           hunchConfig.SumEngineImage,
			"kubearmor-relay-server":   hunchConfig.KubeArmorRelayServerImage,
			"spire-agent":              hunchConfig.SPIREAgentImage,
			"shared-informer-agent":    hunchConfig.SIAImage,
			"policy-enforcement-agent": hunchConfig.PEAImage,
			"feeder-service":           hunchConfig.FeederImage,
			"hardening-agent":          hunchConfig.HardeningAgentImage,
			"discovery-engine":         hunchConfig.DiscoverImage,
		}, nil
	}

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
