package imagescan

import (
	"errors"
	"fmt"
	"os"
	"syscall"

	kubesheildDiscovery "github.com/accuknox/kubeshield/pkg/discovery"
	kubesheildConfig "github.com/accuknox/kubeshield/pkg/scanner/config"
	kubesheildScanner "github.com/accuknox/kubeshield/pkg/scanner/scan"
	"github.com/samber/lo"
	"go.uber.org/zap"
)

// Discovers the running container images and scans the images using the specified tool
func DiscoverAndScan(conf kubesheildConfig.Config, hostName, runtime string) error {
	zapLogger, err := zap.NewProduction()
	if err != nil {
		return fmt.Errorf("failed to initialize logger")
	}

	if zapLogger != nil {
		conf.Log = zapLogger.Sugar()
	}

	defer func() {
		// Ignoring EINVAL errors based on https://github.com/uber-go/zap/issues/328#issuecomment-284337436
		if err := zapLogger.Sync(); err != nil && !errors.Is(err, syscall.EINVAL) {
			fmt.Printf("error: %v\n", err)
		}
	}()

	conf.Images = discoverImages(zapLogger.Sugar(), hostName, runtime)
	if len(conf.Images) == 0 {
		return fmt.Errorf("no images found for scanning")
	}

	// removes duplicate images
	conf.Images = lo.Uniq(conf.Images)
	for i := range conf.Images {
		zapLogger.Info("Discovered Image", zap.String("Name", conf.Images[i].Name), zap.String("Runtime", conf.Images[i].Runtime))
	}
	zapLogger.Info("Images Discovered Successfully", zap.Int("Total number of images:", len(conf.Images)))

	if hostName == "" {
		hostName, _ = os.Hostname()
	}

	// Additional fields added along with the scan results while calling artifact API
	conf.ScanConfig.AdditionalData = map[string]any{"host_name": hostName}

	imageScanner := kubesheildScanner.New(conf)

	// Scans the provided images and sends the result back to saas through the artifact API
	if err := imageScanner.Scan(); err != nil {
		return fmt.Errorf("error while scanning the images")
	}

	zapLogger.Info("Images Scanned Successfully",
		zap.Int("Total Scanned Images", len(conf.Images)),
		zap.String("Tool used for scanning", conf.ScanConfig.ScanTool))

	return nil
}

// Lists the running containers for the provided runtime, if the runtime is empty it will use the default supported runtimes
func discoverImages(logger *zap.SugaredLogger, hostName, runtime string) []kubesheildConfig.Image {
	var (
		runtimes = []string{"docker", "containerd", "cri-o", "nri"}
		images   []kubesheildConfig.Image
	)

	if runtime != "" {
		runtimes = []string{runtime}
	}

	// Fetching images present in all the provided runtimes
	for _, r := range runtimes {
		detectedRuntime, criPath, ok := kubesheildDiscovery.DiscoverNodeRuntime("", r, logger)
		if !ok {
			logger.Errorf("Unable to detect runtime for %s", r)
			continue
		}
		imageList := kubesheildDiscovery.ListRunningImages(detectedRuntime, criPath, kubesheildDiscovery.VM, logger)
		for _, img := range imageList {
			images = append(images, kubesheildConfig.Image{
				Name:    img,
				Runtime: detectedRuntime,
			})
		}
	}
	return images
}
