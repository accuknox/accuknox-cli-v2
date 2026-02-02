package imagescan

import (
	"errors"
	"fmt"
	"os"
	"syscall"
	"time"

	"github.com/accuknox/kubeshield/api/v1beta1"
	kubesheildDiscovery "github.com/accuknox/kubeshield/pkg/discovery"
	httpclient "github.com/accuknox/kubeshield/pkg/scanner/httpClient"
	"github.com/accuknox/kubeshield/pkg/scanner/registrysink"
	kubesheildScanner "github.com/accuknox/kubeshield/pkg/scanner/scan"
	constants "github.com/accuknox/kubeshield/pkg/scanner/utils/constant"
	"github.com/samber/lo"
	"go.uber.org/zap"
)

var (
	NodeType          string
	FlushTo           string
	InsecureConn      bool
	defaultRoutingKey = "registry-scanning"
	CreateRegistryURL string
	scanDate          = time.Now().Format(time.DateOnly)
)

// Discovers the running container images and scans the images using the specified tool
func DiscoverAndScan(conf kubesheildScanner.ScanConfig, hostName, runtime string, onlyRunningContainers, onlyImages bool) error {
	zapLogger, err := zap.NewProduction()
	if err != nil {
		return fmt.Errorf("failed to initialize logger")
	}

	defer func() {
		// Ignoring EINVAL errors based on https://github.com/uber-go/zap/issues/328#issuecomment-284337436
		if err := zapLogger.Sync(); err != nil && !errors.Is(err, syscall.EINVAL) {
			fmt.Printf("error: %v\n", err)
		}
	}()

	// Install trivy if it is not exists
	if !IsTrivyInstalled() {
		if err := installTrivy(); err != nil {
			return fmt.Errorf("error while installing container image scanner: %v", err)
		}
		zapLogger.Info("Dowloaded container image scanner successfully")
		// Remove trivy binary, if it is installed by knoxctl
		defer cleanupInstalledBinaryPath()
	}

	if hostName == "" {
		hostName, _ = os.Hostname()
	}

	// Additional fields added along with the scan results while calling artifact API
	conf.ArtifactConfig.AdditionalData = map[string]any{"host_name": hostName}
	conf.ScanTool = "trivy" // Default scanning tool

	// Passing nil kubernetes.Clientset, because it won't be required incase of VM container Image scanning
	imageScanner := kubesheildScanner.New(conf, nil)
	imageScanner.ScannerHttpClient = httpclient.New()

	if NodeType == "cp" {
		conf.RegistryConfig.RegistryID, conf.RegistryConfig.RegistryName, err = httpclient.CreateRegistry(CreateRegistryURL)
	}

	if NodeType != "" {
		if imageScanner.Sink, err = registrysink.NewSink(defaultRoutingKey); err != nil {
			return err
		}
	}

	// Sets the scanner status as `PENDING` and analyzer status as `INPROGRESS`
	if err := imageScanner.Sink.InitRegistryScan(conf.RegistryConfig.RegistryID, scanDate,
		registrysink.SetAnalysisStatus(constants.RegistryScanStatusInProgress),
		registrysink.SetScanStatus(constants.RegistryScanStatusPending)).
		Flush(); err != nil {
		return err
	}

	imageScanner.ScanConfig.Images = discoverImages(hostName, runtime, onlyRunningContainers, onlyImages, zapLogger.Sugar())
	if len(imageScanner.ScanConfig.Images) == 0 {
		return fmt.Errorf("no images found for scanning")
	}

	// removes duplicate images
	imageScanner.ScanConfig.Images = lo.UniqBy(imageScanner.ScanConfig.Images, func(img v1beta1.Image) string {
		return img.Name
	})

	for i := range imageScanner.ScanConfig.Images {
		zapLogger.Sugar().Infof("Image Name: %s | Runtime: %s", imageScanner.ScanConfig.Images[i].Name, imageScanner.ScanConfig.Images[i].Runtime)

		// Flushing all the Images and setting the status as `Pending`
		if err := imageScanner.Sink.InitImageScan(imageScanner.ScanConfig.Images[i].Name,
			registrysink.SetImageStatus(constants.ImageScanStatusPending)).
			Flush(); err != nil {
			return err
		}
	}

	// Marking the analysis status as `COMPLETED` once all the Images are flushed successfully
	imageScanner.Sink.RegistryData.RegistryScanDetail.AnalysisStatus = constants.RegistryScanStatusCompleted
	if err := imageScanner.Sink.Flush(); err != nil {
		return err
	}

	zapLogger.Info("Images Discovered Successfully", zap.Int("Total number of images:", len(imageScanner.ScanConfig.Images)))

	// Scans the provided images and sends the result back to saas through the artifact API
	if err := imageScanner.Scan(); err != nil {
		return fmt.Errorf("error while scanning the images")
	}

	// Setting the Registry-Scan status as successful
	if err := imageScanner.Sink.InitRegistryScan(conf.RegistryConfig.RegistryID, scanDate,
		registrysink.SetScanStatus(constants.RegistryScanStatusCompleted)).
		Flush(); err != nil {
		return err
	}

	zapLogger.Info("Images Scanned Successfully",
		zap.Int("Total Scanned Images", len(imageScanner.ScanConfig.Images)))

	return nil
}

// Lists the running containers for the provided runtime, if the runtime is empty it will use the default supported runtimes
func discoverImages(hostName, runtime string, onlyRunningContainers, imageOnly bool, logger *zap.SugaredLogger) []v1beta1.Image {
	var (
		runtimes   = []string{"docker", "containerd", "cri-o", "nri"}
		imagesList = []v1beta1.Image{}
	)

	if runtime != "" {
		runtimes = []string{runtime}
	}

	// Fetching images present in all the provided runtimes
	for _, r := range runtimes {
		detectedRuntime, criPath, ok := DiscoverRuntime("", r)
		if !ok {
			logger.Debugf("Unable to detect runtime for %s", r)
			continue
		}

		// If imageOnly flag is enabled, we only discover images; not containers
		if imageOnly {
			// fetch images based on the runtime
			imagesList = append(imagesList, getImages(detectedRuntime, criPath, logger)...)
			continue
		}

		// By default we fetch running containers, unless onlyRunningContainers is set to false
		imagesList = append(imagesList, getContainers(detectedRuntime, criPath, onlyRunningContainers, logger)...)
	}
	return imagesList
}

// fetches the images based on all the provided runtime and cripath
func getImages(runtime string, criPath []string, logger *zap.SugaredLogger) []v1beta1.Image {
	var images = []v1beta1.Image{}
	for _, path := range criPath {
		imageList, err := kubesheildDiscovery.ListImages(runtime, path, kubesheildDiscovery.VM)
		if err != nil {
			logger.Errorf("error while listing the images for runtime %s: %v\n", runtime, err)
			continue
		}
		images = append(images, imageList...)
	}
	return images
}

// fetches the containers based on all the provided runtime and cripath
func getContainers(runtime string, criPath []string, onlyRunningContainers bool, logger *zap.SugaredLogger) []v1beta1.Image {
	var conatainers = []v1beta1.Image{}
	for _, path := range criPath {
		containerList, err := kubesheildDiscovery.ListContainers(runtime, path, kubesheildDiscovery.VM, onlyRunningContainers)
		if err != nil {
			logger.Errorf("error while listing the container images for runtime %s: %v", runtime, err)
			continue
		}
		conatainers = append(conatainers, containerList...)
	}
	return conatainers
}
