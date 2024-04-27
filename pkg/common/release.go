package common

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"os"

	"golang.org/x/mod/semver"
)

type ReleaseMetadata struct {
	CreationTime          string `json:"creation_time"`
	KubeArmorTag          string `json:"kubearmor_tag"`
	KubeArmorRelayTag     string `json:"kubearmor_relay_tag"`
	KubeArmorVMAdapterTag string `json:"kubearmor_vm_adapter_tag"`
	SIATag                string `json:"sia_tag"`
	SIAImage              string `json:"sia_image"`
	PEATag                string `json:"pea_tag"`
	PEAImage              string `json:"pea_image"`
	FeederServiceTag      string `json:"feeder_service_tag"`
	FeederServiceImage    string `json:"feeder_service_image"`
}

var (
	//go:embed release.json
	releaseInfoFile []byte

	ReleaseInfo = make(map[string]ReleaseMetadata, 0)
)

func init() {
	err := json.Unmarshal(releaseInfoFile, &ReleaseInfo)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
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
