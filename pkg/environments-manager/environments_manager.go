package environments_manager

import (
	"errors"
	"io/ioutil"

	"gopkg.in/yaml.v3"
)

// errors
var (
	ErrEnvironmentAlreadyExists = errors.New("environment already exists")
	ErrEnvironmentNotExist      = errors.New("environment does not exist")
)

const (
	configVersion = "1.0"

	DefaultAppsPath = "argocd-apps"
)

type (
	Config struct {
		path         string       // the path from which the config was loaded
		Version      string       `yaml:"version"`
		Environments Environments `yaml:"environments,inline"`
	}

	Environments map[string]Environment

	Environment struct {
		RootApplicationPath string `yaml:"rootAppPath"`
	}
)

func NewConfig(path string) *Config {
	return &Config{
		path:         path,
		Version:      configVersion,
		Environments: make(Environments),
	}
}

// Persist saves the config to file
func (c *Config) Persist() error {
	data, err := yaml.Marshal(c)
	if err != nil {
		return err
	}

	return ioutil.WriteFile(c.path, data, 0644)
}

// AddEnvironmentP adds a new environment and persists the config object
func (c *Config) AddEnvironmentP(name string, env Environment) error {
	if _, exists := c.Environments[name]; exists {
		return ErrEnvironmentAlreadyExists
	}

	c.Environments[name] = env

	return c.Persist()
}

// DeleteEnvironmentP deletes an environment and persists the config object
func (c *Config) DeleteEnvironmentP(name string, env Environment) error {
	if _, exists := c.Environments[name]; !exists {
		return ErrEnvironmentNotExist
	}

	delete(c.Environments, name)

	return c.Persist()
}

// LoadConfig loads the config from the specified path
func LoadConfig(path string) (*Config, error) {
	data, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}

	c := new(Config)
	c.path = path
	return c, yaml.Unmarshal(data, c)
}
