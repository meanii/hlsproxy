package config

import (
	"fmt"
	"io"
	"os"
	"path"

	"go.uber.org/zap"
	"gopkg.in/yaml.v3"
)

type GlobalConfig struct {
	Name    string `yaml:"name"`
	Version string `yaml:"version"`
	Config  struct {
		OriginServer struct {
			URL string `yaml:"url"`
		} `yaml:"orgin_server"`
		Ffmpeg struct {
			Bin string `yaml:"bin"`
		} `yaml:"ffmpeg"`
		Cache struct {
			Dirname string `yaml:"dirname"`
		} `yaml:"cache"`
	} `yaml:"config"`
}

var GlobalConfigInstance *GlobalConfig

func New(filename string) *GlobalConfig {
	cfg := &GlobalConfig{}
	cfginstance := cfg.load(filename)
	return cfginstance
}

func (gc *GlobalConfig) load(filename string) *GlobalConfig {
	wd, err := os.Getwd()
	if err != nil {
		zap.S().Fatalf("coudnt get wording dir, Error: %s", err)
	}
	configpath := path.Join(wd, filename)
	configbyte, _ := gc.readConfigFile(configpath)
	config := gc.unmarshel(configbyte)

	// adding to singleton instance
	GlobalConfigInstance = config

	zap.S().Infof("config file loaded: %s", filename)
	return config
}

func (gc *GlobalConfig) readConfigFile(filepath string) ([]byte, error) {
	file, err := os.Open(filepath)
	if err != nil {
		zap.S().Fatalf("coudnt open config file filename:%s, Error: %s", filepath, err)
	}
	configbyte, err := io.ReadAll(file)
	if err != nil {
		zap.S().Errorf("coudnt read the file file: %s, Error: %s", filepath, err)
	}
	return configbyte, nil
}

func (gc *GlobalConfig) unmarshel(config []byte) *GlobalConfig {
	cnf := GlobalConfig{}
	fmt.Printf("config %s", config)
	err := yaml.Unmarshal(config, cnf)
	if err != nil {
		zap.S().Fatalf("failed to decode yaml, Error: %s", err)
	}
	return &cnf
}
