package discoveryengine

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/accuknox/accuknox-cli-v2/pkg/common"
	"gopkg.in/yaml.v2"
)

type ConfigMapStruct struct {
	name      string
	namespace string
	data      map[string]string
}

func GetConfigmap(ns string) ([]ConfigMapStruct, error) {
	var configMaps []ConfigMapStruct
	configMapDir := common.ConfigMapDirPath
	files, err := os.ReadDir(configMapDir)
	if err != nil {
		return configMaps, fmt.Errorf("error reading the directory. error: %v", err)
	}

	for _, file := range files {
		if file.IsDir() {
			continue
		}

		fileName := file.Name()
		if fileName != filepath.Base(fileName) {
			return configMaps, fmt.Errorf("invalid file name: %s", fileName)
		}

		filePath := filepath.Join(configMapDir, fileName)
		filePath = filepath.Clean(filePath)

		data, err := os.ReadFile(filePath)
		if err != nil {
			return configMaps, fmt.Errorf("error reading the yaml file. error: %v", err)
		}

		var configMapData map[string]string
		if err := yaml.Unmarshal(data, &configMapData); err != nil {
			return configMaps, fmt.Errorf("error parsing yaml file %s. error: %v", file.Name(), err)
		}
		configMapName := strings.TrimSuffix(file.Name(), ".yaml")
		configMap := ConfigMapStruct{
			name:      configMapName,
			namespace: ns,
			data:      configMapData,
		}

		configMaps = append(configMaps, configMap)
	}

	return configMaps, nil
}
