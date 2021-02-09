package environments_manager

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"path/filepath"

	"github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	"gopkg.in/yaml.v3"
)

// errors
var (
	ErrEnvironmentAlreadyExists = errors.New("environment already exists")
	ErrEnvironmentNotExist      = errors.New("environment does not exist")
)

const (
	configVersion = "1.0"

	configFileName  = "codefresh.yaml"
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
		c                   *Config
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

	return ioutil.WriteFile(filepath.Join(c.path, configFileName), data, 0644)
}

// AddEnvironmentP adds a new environment and persists the config object
func (c *Config) AddEnvironmentP(name string, env Environment) error {
	if _, exists := c.Environments[name]; exists {
		return ErrEnvironmentAlreadyExists
	}

	env.c = c
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
	data, err := ioutil.ReadFile(filepath.Join(path, configFileName))
	if err != nil {
		return nil, err
	}

	c := new(Config)
	c.path = path
	if err = yaml.Unmarshal(data, c); err != nil {
		return nil, err
	}
	for _, e := range c.Environments {
		e.c = c
	}

	return c, nil
}

func (e Environment) GetAppByName(appName string) (*v1alpha1.Application, error) {
	rootApp, err := getAppFromFile(filepath.Join(e.c.path, e.RootApplicationPath))
	if err != nil {
		return nil, err
	}

	app, err := e.getAppByNameRecurse(rootApp, appName)
	if err != nil {
		return nil, err
	}
	if app == nil {
		return nil, fmt.Errorf("app not found")
	}
	return app, nil
}

func (e Environment) getAppByNameRecurse(parentApp *v1alpha1.Application, appName string) (*v1alpha1.Application, error) {
	if parentApp.Name == appName {
		return parentApp, nil
	}

	appsDir := parentApp.Spec.Source.Path // check if it's not in this repo
	filenames, err := filepath.Glob(filepath.Join(e.c.path, appsDir, "*.yaml"))
	if err != nil {
		return nil, err
	}

	for _, f := range filenames {
		childApp, err := getAppFromFile(f)
		if err != nil {
			fmt.Printf("blah blah %s", f)
		}

		return e.getAppByNameRecurse(childApp, appName)
	}

	return nil, nil
}

func getAppFromFile(path string) (*v1alpha1.Application, error) {
	data, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}

	d := yaml.NewDecoder(bytes.NewBuffer(data))
	app := &v1alpha1.Application{}
	for {
		err = d.Decode(app)
		if err == io.ErrUnexpectedEOF {
			return nil, err
		}
		if err == nil {
			return app, nil
		}
	}
}
