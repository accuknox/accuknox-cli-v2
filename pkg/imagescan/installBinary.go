package imagescan

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
)

var defaultInstallationPath = filepath.Join(os.Getenv("HOME"), ".accuknox-config", "container-scanner")

// Download and install the provided binary
func DownloadAndInstallBinary(url, targetDir string) error {

	// Download the tar.gz file
	tarReader, err := downloadTarGz(url)
	if err != nil {
		return err
	}

	defer tarReader.Close()

	// Decompress and untar the downloaded tar.gz file
	result, err := untar(tarReader, targetDir)
	if err != nil {
		return err
	}

	// copy the executable to the provided path
	return installBinary(result, targetDir)
}

// Creates the file in the provided path and sets the file permission to executable.
func installBinary(binary io.Reader, binaryPath string) error {
	outFile, err := os.Create(path.Clean(binaryPath))
	if err != nil {
		return err
	}

	defer outFile.Close()

	if _, err := io.Copy(outFile, binary); err != nil {
		return err
	}

	// Make it executable
	err = os.Chmod(binaryPath, 0700) // #nosec G302
	if err != nil {
		return err
	}
	return nil
}

// Download the tar.gz file
func downloadTarGz(rawURL string) (io.ReadCloser, error) {
	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return nil, err
	}
	resp, err := http.Get(parsedURL.String())
	if err != nil {
		return nil, fmt.Errorf("failed to download file: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		if err := resp.Body.Close(); err != nil {
			return nil, err
		}
		return nil, fmt.Errorf("unexpected HTTP status: %s", resp.Status)
	}

	return resp.Body, nil
}

// Decompress the tar.gz file
func untar(src io.Reader, binaryPath string) (io.Reader, error) {

	gz, err := gzip.NewReader(src)
	if err != nil {
		return nil, err
	}
	return unarchiveTar(gz, binaryPath)
}

// Untar the tar archive for the provided binary
func unarchiveTar(gzStream io.Reader, binaryPath string) (io.Reader, error) {
	tarReader := tar.NewReader(gzStream)
	_, binaryName := filepath.Split(binaryPath)
	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}

		// Return only the untar of the provided binary
		if header.Typeflag == tar.TypeReg && filepath.Base(header.Name) == binaryName {
			return tarReader, nil
		}
	}
	return nil, fmt.Errorf("%s binary not found in tar.gz", binaryName)
}

// Creating the binary path, if not exists
func getBinaryPath() (string, error) {
	if err := os.MkdirAll(defaultInstallationPath, 0750); err != nil {
		return "", err
	}

	// Including the newly created path to the $PATH variable
	newPath := fmt.Sprintf("%s:%s", defaultInstallationPath, os.Getenv("PATH"))
	if err := os.Setenv("PATH", newPath); err != nil {
		return "", err
	}
	return defaultInstallationPath, nil
}

// Delete Installed binary
func cleanupInstalledBinaryPath() error {
	return os.RemoveAll(defaultInstallationPath)
}
