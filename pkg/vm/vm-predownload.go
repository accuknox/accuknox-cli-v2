package vm

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"time"

	cm "github.com/accuknox/accuknox-cli-v2/pkg/common"
	"github.com/accuknox/accuknox-cli-v2/pkg/logger"
	"github.com/accuknox/accuknox-cli-v2/pkg/onboard"
	"github.com/docker/docker/api/types/image"

	"github.com/pterm/pterm"
	"github.com/regclient/regclient"
	"github.com/regclient/regclient/types/platform"
	"github.com/regclient/regclient/types/ref"
)

type DownloadOptions struct {
	Arch                       []string
	SavePath                   string
	VMMode                     []onboard.VMMode
	Version                    string
	Registry                   string
	RegistryConfigPath         string
	PreserveUpstream           bool
	InsecureRegistryConnection bool
	HttpRegistryConnection     bool
	Debug                      bool
	ImageVersions              *onboard.ImageVersions
}

func (o *DownloadOptions) Download() error {

	basePath, err := getBasePath(o.SavePath)
	if err != nil {
		return err
	}

	releaseInfo, ok := cm.ReleaseInfo[o.Version]
	if !ok {
		var version string
		version, releaseInfo = cm.GetLatestReleaseInfo()
		logger.Warn("unknown image tag %s, using latest version %s instead", o.Version, version)
		o.Version = version
	}
	tableData := pterm.TableData{
		{"Mode", "Arch", "Downloaded", "Skipped", "Version"},
	}

	for _, mode := range o.VMMode {
		for _, arch := range o.Arch {
			dir, err := makeDirPaths(basePath, string(mode), arch)
			if err != nil {
				return err
			}

			cc := onboard.ClusterConfig{
				Mode: mode,
			}

			images, err := o.getImageDetails(arch, releaseInfo, &cc)
			if err != nil {
				return err
			}
			var data []string
			switch mode {
			case onboard.VMMode_Systemd:
				downloaded, skipped := downloadSystemdAgents(cc, dir, arch, images)
				data = []string{string(mode), arch, fmt.Sprintf("%d", downloaded), fmt.Sprintf("%d", skipped), o.Version}
			case onboard.VMMode_Docker:
				downloaded, skipped := downloadDockerAgents(arch, dir, images)
				data = []string{string(mode), arch, fmt.Sprintf("%d", downloaded), fmt.Sprintf("%d", skipped), o.Version}
			}

			tableData = append(tableData, pterm.TableData{data}...)

			outFileName := fmt.Sprintf("%v-%v.tar.gz", string(mode), arch)
			if err := compressDirectory(basePath, dir, outFileName); err != nil {
				return err
			}

		}
	}
	pterm.Println()

	// #nosec G104 - false positive
	pterm.DefaultTable.WithHasHeader(true).WithHeaderRowSeparator("_").WithData(tableData).WithBoxed(true).Render()
	pterm.Println()

	root := &pterm.TreeNode{
		Text: basePath,
	}
	err = buildTree(basePath, root, o.Debug)
	if err != nil {
		return err
	}
	// #nosec G104 - false positive
	pterm.DefaultTree.WithRoot(*root).Render()

	return nil
}

func makeDirPaths(basePath, mode, arch string) (string, error) {
	dir := filepath.Join(basePath, string(mode), arch)
	return dir, os.MkdirAll(dir, 0750)
}

func getBasePath(path string) (string, error) {
	if path != "" {
		return path, nil
	}
	wd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	return filepath.Join(wd, "accuknox"), nil
}

func pullAndSave(dockerImage, arch, outFileName string) error {

	if !shouldDownload(outFileName) {
		time.Sleep(time.Second)
		return nil
	}

	dClient, err := onboard.CreateDockerClient()
	if err != nil {
		return err
	}

	ctx := context.Background()

	inspect, err := dClient.DistributionInspect(ctx, dockerImage, "")
	if err != nil {
		fmt.Println("Error inspecting image:", err)
		return err
	}

	notFound := true
	for _, platform := range inspect.Platforms {
		if platform.Architecture == arch {
			notFound = false
			break
		}
	}

	if notFound {
		return fmt.Errorf("architecture %s not found ", arch)
	}

	rc := regclient.New()
	imgRef, err := ref.New(dockerImage)
	if err != nil {
		return err
	}

	manifest, err := rc.ManifestGet(ctx, imgRef, regclient.WithManifestPlatform(platform.Platform{
		Architecture: arch,
		OS:           runtime.GOOS,
	}))
	if err != nil {
		return err
	}
	digest := manifest.GetDescriptor().Digest.String()
	digestRef := fmt.Sprintf("%s@%s", imgRef.CommonName(), digest)

	pullOptions := image.PullOptions{
		Platform: runtime.GOOS + "/" + arch,
	}

	pullReader, err := dClient.ImagePull(ctx, digestRef, pullOptions)
	if err != nil {
		return fmt.Errorf("pull failed: %w", err)
	}
	defer pullReader.Close()

	_, err = io.Copy(io.Discard, pullReader)
	if err != nil {
		return fmt.Errorf("reading pull output failed: %w", err)
	}

	saveReader, err := dClient.ImageSave(ctx, []string{digestRef})
	if err != nil {
		return err
	}
	defer saveReader.Close()

	tmpFile := outFileName + ".tmp"

	// #nosec G304 -- false positive
	outFile, err := os.Create(tmpFile)
	if err != nil {
		return err
	}
	gzWriter := gzip.NewWriter(outFile)

	_, err = io.Copy(gzWriter, saveReader)
	if err != nil {
		if err := gzWriter.Close(); err != nil {
			return err
		}
		if err := outFile.Close(); err != nil {
			return err
		}
		if err := os.Remove(tmpFile); err != nil {
			return err
		}
		return fmt.Errorf("reading pull output failed: %w", err)
	}

	if err := gzWriter.Close(); err != nil {
		return err
	}
	if err := outFile.Close(); err != nil {
		return err
	}

	if err := os.Rename(tmpFile, outFileName); err != nil {
		return err
	}

	return nil
}

func (o *DownloadOptions) getImageDetails(arch string, releaseInfo cm.ReleaseMetadata, cc *onboard.ClusterConfig) (map[string]string, error) {

	tagSuffix := "_" + runtime.GOOS + "-" + arch

	err := cc.PopulateImageDetails(releaseInfo,
		o.ImageVersions,
		o.Registry,
		o.RegistryConfigPath,
		tagSuffix,
		o.PreserveUpstream,
		o.InsecureRegistryConnection,
		o.HttpRegistryConnection)
	if err != nil {
		return nil, err
	}

	return map[string]string{
		"kubearmor":                cc.KubeArmorImage,
		"kubearmor-init":           cc.KubeArmorInitImage,
		"kubearmor-vm-adapter":     cc.KubeArmorVMAdapterImage,
		"kubearmor-relay-server":   cc.KubeArmorRelayServerImage,
		"spire-agent":              cc.SPIREAgentImage,
		"wait-for-it":              cc.WaitForItImage,
		"shared-informer-agent":    cc.SIAImage,
		"policy-enforcement-agent": cc.PEAImage,
		"feeder-service":           cc.FeederImage,
		"rabbitMQ":                 cc.RMQImage,
		"discover-agent":           cc.DiscoverImage,
		"summary-engine":           cc.SumEngineImage,
		"hardening-agent":          cc.HardeningAgentImage,
		"rra":                      cc.RRAImage,
	}, nil

}

func downloadSystemdAgents(
	cc onboard.ClusterConfig,
	dir, arch string,
	images map[string]string,
) (int, int) {

	skipped := 0
	downloaded := 0

	p, _ := pterm.DefaultProgressbar.WithTotal(len(images)).WithTitle("Downloading binaries").WithRemoveWhenDone(true).Start()

	for image, binaryImage := range images {

		p.UpdateTitle(fmt.Sprintf("Downloading %s [%s] binary", image, arch))

		if binaryImage == "" {
			skipped++
			pterm.Warning.Printf("skipping %v [%v]: binary not required\n", image, arch)
			p.Increment()
			continue
		}
		imgTag := strings.Split(binaryImage, ":")
		if len(imgTag) != 2 {
			skipped++
			pterm.Warning.Printf("skipping %v [%v]: binary tag is empty\n", image, arch)
			p.Increment()
			continue
		}

		_, err := cc.DownloadAgent(image, imgTag[0], imgTag[1], dir)
		if err != nil {
			pterm.Error.Printf("error downloading binary %s [%v]: %v\n", image, arch, err)
			p.Increment()
			skipped++
			continue
		}
		downloaded++
		p.Increment()
	}
	// #nosec G104 -- false positive
	p.Stop()
	return downloaded, skipped

}

func downloadDockerAgents(
	arch, dir string,
	images map[string]string,
) (int, int) {

	skipped := 0
	downloaded := 0
	p, _ := pterm.DefaultProgressbar.WithTotal(len(images)).WithTitle("Downloading images").WithRemoveWhenDone(true).Start()

	for image, dockerImage := range images {

		p.UpdateTitle(fmt.Sprintf("Downloading %s [%s] image", image, arch))

		if dockerImage == "" {
			skipped++
			pterm.Warning.Printf("skipping %v [%v]: image is empty\n", image, arch)
			p.Increment()
			continue
		}
		imgTag := strings.Split(dockerImage, ":")
		if len(imgTag) != 2 {
			pterm.Warning.Printf("skipping %v [%v]: image tag is empty\n", image, arch)
			p.Increment()
			skipped++
			continue
		}
		outFileName := filepath.Join(dir, image+".tar.gz")
		if err := pullAndSave(dockerImage, arch, outFileName); err != nil {
			pterm.Error.Printf("failed to download image %s [%v]: %v\n", dockerImage, arch, err)
			p.Increment()
			skipped++
			continue
		}
		downloaded++
		p.Increment()
	}
	// #nosec G104 -- false positive
	p.Stop()
	return downloaded, skipped

}

func buildTree(path string, parent *pterm.TreeNode, debug bool) error {
	entries, err := os.ReadDir(path)
	if err != nil {
		return err
	}

	for _, entry := range entries {

		if !debug && !entry.IsDir() && !shouldShow(entry.Name()) {
			continue
		}

		parent.Children = append(parent.Children, pterm.TreeNode{
			Text: entry.Name(),
		})

		child := &parent.Children[len(parent.Children)-1]

		if entry.IsDir() {
			err := buildTree(filepath.Join(path, entry.Name()), child, debug)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func compressDirectory(baseDir, sourceDir, fileName string) error {

	outFileName := filepath.Join(baseDir, fileName)

	if !shouldDownload(outFileName) {
		time.Sleep(time.Second)
		return nil
	}

	outFile, err := os.Create(filepath.Clean(outFileName))
	if err != nil {
		return err
	}
	defer outFile.Close()
	gz := gzip.NewWriter(outFile)
	defer gz.Close()
	tw := tar.NewWriter(gz)
	defer tw.Close()

	return filepath.Walk(sourceDir, func(file string, fi os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		relPath, err := filepath.Rel(baseDir, file)
		if err != nil {
			return err
		}

		header, err := tar.FileInfoHeader(fi, "")
		if err != nil {
			return err
		}
		header.Name = relPath

		if err := tw.WriteHeader(header); err != nil {
			return err
		}

		if fi.IsDir() {
			return nil
		}

		// #nosec G122 -- parameters are controlled
		f, err := os.Open(filepath.Clean(file))
		if err != nil {
			return err
		}
		defer f.Close()

		_, err = io.Copy(tw, f)
		return err
	})

}

func shouldShow(path string) bool {
	return slices.ContainsFunc([]string{
		"systemd-arm64.tar.gz",
		"systemd-amd64.tar.gz",
		"docker-arm64.tar.gz",
		"docker-amd64.tar.gz",
	}, func(s string) bool {
		return strings.Contains(path, s)
	})
}

func shouldDownload(filename string) bool {
	info, err := os.Stat(filename)
	if err != nil {
		return true
	}

	if info.Size() > 0 {
		f, err := os.Open(filepath.Clean(filename))
		if err != nil {
			return true
		}
		defer f.Close()

		gz, err := gzip.NewReader(f)
		if err == nil {
			defer gz.Close()
			return false
		}
	}

	return true
}
