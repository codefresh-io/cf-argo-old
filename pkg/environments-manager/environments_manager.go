package environments_manager

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	"gopkg.in/yaml.v3"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	kyaml "sigs.k8s.io/yaml"
)

// errors
var (
	ErrEnvironmentAlreadyExists = errors.New("environment already exists")
	ErrEnvironmentNotExist      = errors.New("environment does not exist")

	yamlSeparator = regexp.MustCompile(`\n---`)
)

const (
	configVersion = "1.0"

	configFileName  = "codefresh.yaml"
	labelsCfName    = "cf-name"
	labelsManagedBy = "ent-managed-by"
	DefaultAppsPath = "argocd-apps"
)

type (
	Config struct {
		path         string                  // the path from which the config was loaded
		Version      string                  `yaml:"version"`
		Environments map[string]*Environment `yaml:"environments"`
	}
	Environment struct {
		c                   *Config
		RootApplicationPath string `yaml:"rootAppPath"`
	}

	Application struct {
		*v1alpha1.Application
		path string
	}
)

func NewConfig(path string) *Config {
	return &Config{
		path:         path,
		Version:      configVersion,
		Environments: make(map[string]*Environment),
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
func (c *Config) AddEnvironmentP(name string, env *Environment) error {
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

func (c *Config) FirstEnv() *Environment {
	for _, env := range c.Environments {
		return env
	}
	return nil
}

// LoadConfig loads the config from the specified path
func LoadConfig(path string) (*Config, error) {
	data, err := ioutil.ReadFile(filepath.Join(path, configFileName))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("config file does not exist: %s", path)
		}
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

func (e *Environment) LeafApps() ([]*Application, error) {
	rootApp, err := e.getRootApp()
	if err != nil {
		return nil, err
	}

	return e.leafAppsRecurse(rootApp)
}

func (e *Environment) leafAppsRecurse(root *Application) ([]*Application, error) {
	filenames, err := filepath.Glob(filepath.Join(e.c.path, root.Spec.Source.Path, "*.yaml"))
	if err != nil {
		return nil, err
	}

	isLeaf := true
	res := []*Application{}
	for _, f := range filenames {
		childApp, err := getAppFromFile(f)
		if err != nil {
			fmt.Printf("file is not an argo-cd application manifest %s\n", f)
			continue
		}

		if childApp != nil {
			isLeaf = false
			childRes, err := e.leafAppsRecurse(childApp)
			if err != nil {
				return nil, err
			}
			res = append(res, childRes...)
		}
	}
	if isLeaf {
		res = append(res, root)
	}

	return res, nil
}

func (e *Environment) getRootApp() (*Application, error) {
	return getAppFromFile(filepath.Join(e.c.path, e.RootApplicationPath))
}

func (e *Environment) GetAppByName(appName string) (*Application, error) {
	rootApp, err := e.getRootApp()
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

func (e *Environment) getAppByNameRecurse(root *Application, appName string) (*Application, error) {
	if GetAppCfName(root) == appName {
		return root, nil
	}

	appsDir := root.Spec.Source.Path // check if it's not in this repo
	filenames, err := filepath.Glob(filepath.Join(e.c.path, appsDir, "*.yaml"))
	if err != nil {
		return nil, err
	}

	for _, f := range filenames {
		app, err := getAppFromFile(f)
		if err != nil || app == nil {
			// not an argocd app - ignore
			continue
		}

		if getAppManagedBy(app) != "codefresh.io" {
			continue
		}

		res, err := e.getAppByNameRecurse(app, appName)
		if err != nil || res != nil {
			return res, err
		}
	}

	return nil, nil
}

func GetAppCfName(app *Application) string {
	return getAppLabelValue(app, labelsCfName)
}

func getAppManagedBy(app *Application) string {
	return getAppLabelValue(app, labelsManagedBy)
}

func getAppLabelValue(app *Application, label string) string {
	if app.Labels == nil {
		return ""
	}

	return app.Labels[label]
}

func getAppFromFile(path string) (*Application, error) {
	data, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}

	for _, text := range yamlSeparator.Split(string(data), -1) {
		if strings.TrimSpace(text) == "" {
			continue
		}
		u := &unstructured.Unstructured{}
		err := kyaml.Unmarshal([]byte(text), u)
		if err != nil {
			return nil, err
		}

		if u.GetKind() == "Application" {
			data, err := kyaml.YAMLToJSON([]byte(text))
			if err != nil {
				return nil, err
			}
			app := &v1alpha1.Application{}
			return &Application{app, path}, json.Unmarshal(data, app)
		}
	}

	return nil, nil
}

func (a *Application) Save() error {
	json, err := json.Marshal(a)
	if err != nil {
		return err
	}

	data, err := kyaml.JSONToYAML([]byte(json))
	if err != nil {
		return err
	}

	return ioutil.WriteFile(a.path, data, 0644)
}
