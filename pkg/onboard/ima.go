package onboard

import (
	"fmt"
	"strings"

	"github.com/accuknox/accuknox-cli-v2/pkg/logger"
	"golang.org/x/mod/semver"
)

// KubeArmor added the enableIMA configuration option in v1.6.4.
const minimumIMAVersion = "v1.6.4"

// validateIMA checks the resolved image, including any custom image/version
// overrides, before generating KubeArmor configuration.
func (cc *ClusterConfig) validateIMA() {
	if !cc.EnableIMA {
		return
	}

	image := cc.KubeArmorImage
	tagIndex := strings.LastIndex(image, ":")
	if strings.Contains(image, "@") || tagIndex <= strings.LastIndex(image, "/") || tagIndex == -1 {
		logger.Warn("Cannot determine KubeArmor version from image %q; disabling IMA. IMA requires KubeArmor %s or newer.", image, minimumIMAVersion)
		cc.EnableIMA = false
		return
	}

	// Systemd package tags omit 'v' and include an OS/architecture suffix.
	version := ensurePrefix(image[tagIndex+1:])
	fmt.Printf("version: %v\n", version)
	if !semver.IsValid(version) {
		logger.Warn("Cannot verify IMA support for KubeArmor image %q; disabling IMA. Use a versioned image with KubeArmor %s or newer.", image, minimumIMAVersion)
		cc.EnableIMA = false
		return
	}
	if semver.Compare(version, minimumIMAVersion) < 0 {
		logger.Warn("KubeArmor %s does not support IMA; disabling IMA. IMA requires KubeArmor %s or newer.", version, minimumIMAVersion)
		cc.EnableIMA = false
	}
}
