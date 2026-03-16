package config

import (
	"fmt"
	"io"
	"os"

	"gopkg.in/yaml.v3"
)

func loadYAMLFile(path string, out any) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()

	decoder := yaml.NewDecoder(file)
	decoder.KnownFields(true)

	if err := decoder.Decode(out); err != nil {
		return fmt.Errorf("decode YAML: %w", err)
	}

	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		if err == nil {
			return fmt.Errorf("multiple YAML documents are not supported")
		}
		return fmt.Errorf("decode trailing YAML: %w", err)
	}

	return nil
}

func LoadRoot(path string) (RootConfig, error) {
	var cfg RootConfig
	if err := loadYAMLFile(path, &cfg); err != nil {
		return RootConfig{}, fmt.Errorf("load root config %q: %w", path, err)
	}
	return cfg, nil
}

func LoadCoordinator(path string) (CoordinatorConfig, error) {
	var cfg CoordinatorConfig
	if err := loadYAMLFile(path, &cfg); err != nil {
		return CoordinatorConfig{}, fmt.Errorf("load coordinator config %q: %w", path, err)
	}
	return cfg, nil
}

func LoadNode(path string) (NodeConfig, error) {
	var cfg NodeConfig
	if err := loadYAMLFile(path, &cfg); err != nil {
		return NodeConfig{}, fmt.Errorf("load node config %q: %w", path, err)
	}
	return cfg, nil
}
