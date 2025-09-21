package config

import (
	"fmt"
	"os"
	"strings"

	godotenv "github.com/joho/Godotenv"
	"github.com/spf13/viper"
)

type AccuKnoxConfig struct {
	CWPP_URL    string
	CSPM_URL    string
	TENANT_ID   string
	TOKEN       string
	CONFIG_FILE string
}

var Cfg AccuKnoxConfig

func LoadConfig(configFile string) error {
	viper.AutomaticEnv()
	cfgFile := os.Getenv("ACCUKNOX_CFG")
	if cfgFile == "" {
		cfgFile = os.ExpandEnv(configFile)
	}

	// to load shell-styled configuration
	if err := godotenv.Load(cfgFile); err != nil {
		return fmt.Errorf("error while loading config file: %v", err)
	}

	baseURL := viper.GetString("BASE_URL")
	Cfg.CWPP_URL = strings.ReplaceAll(viper.GetString("CWPP_URL"), "$BASE_URL", baseURL)
	Cfg.CSPM_URL = strings.ReplaceAll(viper.GetString("CSPM_URL"), "$BASE_URL", baseURL)
	Cfg.TENANT_ID = viper.GetString("TENANT_ID")
	Cfg.TOKEN = viper.GetString("TOKEN")

	return nil
}

func SetConfig(cwpp, cspm, token, tenant_id string) {
	if cwpp != "" {
		Cfg.CWPP_URL = cwpp
	}
	if cspm != "" {
		Cfg.CSPM_URL = cspm
	}
	if token != "" {
		Cfg.TOKEN = token
	}
	if tenant_id != "" {
		Cfg.TENANT_ID = tenant_id
	}
}
