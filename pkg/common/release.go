package common

import (
	_ "embed"
	"encoding/json"

	"golang.org/x/mod/semver"
)

type ImageTags struct {
	KubeArmorTag          string `json:"kubearmor_tag"`
	KubeArmorRelayTag     string `json:"kubearmor_relay_tag"`
	KubeArmorVMAdapterTag string `json:"kubearmor_vm_adapter_tag"`
	SIATag                string `json:"sia_tag"`
	PEATag                string `json:"pea_tag"`
	FeederServiceTag      string `json:"feeder_service_tag"`
}

var (
	//go:embed release.json
	releaseInfoFile []byte

	ReleaseInfo = make(map[string]ImageTags, 0)
)

func init() {
	err := json.Unmarshal(releaseInfoFile, &ReleaseInfo)
	if err != nil {
		panic(err)
	}
}

func GetLatestReleaseInfo() (string, ImageTags) {
	latestRelease := "v0.0.0"
	for v := range ReleaseInfo {
		if semver.Compare(v, latestRelease) > 0 {
			latestRelease = v
		}
	}

	return latestRelease, ReleaseInfo[latestRelease]
}
