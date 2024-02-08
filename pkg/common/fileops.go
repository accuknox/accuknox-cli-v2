package common

import (
	"fmt"
	"os"
	"path/filepath"
)

func CleanAndRead(filePath string) ([]byte, error) {
	cleanPath := filepath.Clean(filePath)

	data, err := os.ReadFile(cleanPath)
	if err != nil {
		return nil, err
	}

	return data, nil
}

func CleanAndWrite(filePath string, data []byte) error {
	cleanPath := filepath.Clean(filePath)

	err := os.WriteFile(cleanPath, data, 0600)
	if err != nil {
		return err
	}

	return nil
}

func CleanAndCreate(filePath string) (*os.File, error) {
	cleanPath := filepath.Clean(filePath)
	dirPath := filepath.Dir(cleanPath)

	if err := os.MkdirAll(dirPath, 0750); err != nil {
		return nil, fmt.Errorf("failed to create a new directory: %v", err)
	}

	file, err := os.OpenFile(cleanPath, os.O_WRONLY|os.O_CREATE|os.O_CREATE, 0600)
	if err != nil {
		return nil, fmt.Errorf("failed to open the filepath: %v", err)
	}

	return file, nil
}
