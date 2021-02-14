package environments_manager

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	"github.com/ghodss/yaml"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	kustomize "sigs.k8s.io/kustomize/api/types"
)

// errors
var (
	ErrEnvironmentAlreadyExists = errors.New("environment already exists")
	ErrEnvironmentNotExist      = errors.New("environment does not exist")
	ErrAppNotFound              = errors.New("app not found")

	yamlSeparator = regexp.MustCompile(`\n---`)

	rootAppName       = "root"
	componentsAppName = "components"
	entitiesAppName   = "entities"
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
		Version      string                  `json:"version"`
		Environments map[string]*Environment `json:"environments"`
	}
	Environment struct {
		c                   *Config
		RootApplicationPath string `json:"rootAppPath"`
	}

	Application struct {
		*v1alpha1.Application
		parent *Application
		path   string
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

// AddEnvironmentP adds a new environment, copies all of the argocd apps to the relative
// location in the repository that c is managing, and persists the config object
func (c *Config) AddEnvironmentP(name string, env *Environment) error {
	if _, exists := c.Environments[name]; exists {
		return fmt.Errorf("%w: %s", ErrEnvironmentAlreadyExists, name)
	}

	// copy all of the argocd apps to the correct location in the destination repo
	if err := c.installEnv(name, env); err != nil {
		return err
	}

	env.c = c
	c.Environments[name] = env

	return c.Persist()
}

func (c *Config) installEnv(envName string, env *Environment) error {
	lapps, err := env.LeafApps()
	if err != nil {
		return err
	}

	for _, la := range lapps {
		if err = c.installApp(la); err != nil {
			return err
		}
	}

	return nil
}

func (c *Config) GetAppByName(appName string) (*Application, error) {
	var err error
	var app *Application

	for _, e := range c.Environments {
		app, err = e.GetAppByName(appName)
		if err != nil && !errors.Is(err, ErrAppNotFound) {
			return nil, err
		}
		if app != nil {
			return app, nil
		}
	}

	return app, err
}

func (c *Config) installApp(app *Application) error {
	refApp, err := c.GetAppByName(app.CfName())
	if err != nil {
		if !errors.Is(err, ErrAppNotFound) {
			return err
		}
		refApp = &Application{
			path:        app.path,
			Application: app.DeepCopy(),
		}
	}

}

func (c *Config) copyEnv(envName string, env *Environment) error {
	destArgoAppsDir := ""
	srcArgoAppsDir := filepath.Dir(env.RootApplicationPath)

	if len(c.Environments) != 0 {
		destArgoAppsDir = filepath.Dir(c.FirstEnv().RootApplicationPath)
	} else {
		destArgoAppsDir = filepath.Dir(env.RootApplicationPath)
	}

	if err := env.moveApp(componentsAppName, filepath.Join(destArgoAppsDir, envName, componentsAppName)); err != nil {
		return err
	}
	if err := env.moveApp(entitiesAppName, filepath.Join(destArgoAppsDir, envName, entitiesAppName)); err != nil {
		return err
	}
	if err := env.moveApp(rootAppName, filepath.Join(destArgoAppsDir, envName)); err != nil {
		return err
	}

	copyFiles := []string{fmt.Sprintf("%s.yaml", envName), fmt.Sprintf("%s-project.yaml", envName), envName}
	for _, fn := range copyFiles {
		if err := os.Rename(filepath.Join(env.c.path, srcArgoAppsDir, fn), filepath.Join(c.path, destArgoAppsDir, fn)); err != nil {
			return err
		}
	}

	env.RootApplicationPath = fmt.Sprintf("./%s/%s.yaml", destArgoAppsDir, envName)

	return nil
}

func (c *Environment) installRoot(envName string, env *Environment) error {
	return nil
}

func (e *Environment) moveApp(appName, dst string) error {
	app, err := e.GetAppByName(appName)
	if err != nil {
		return err
	}
	app.SetSrcPath(dst)

	return app.Save()
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
		childApp, err := getAppFromFile(f, root)
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
	return getAppFromFile(filepath.Join(e.c.path, e.RootApplicationPath), nil)
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
		return nil, fmt.Errorf("%w: %s", ErrAppNotFound, appName)
	}
	return app, nil
}

func (e *Environment) getAppByNameRecurse(root *Application, appName string) (*Application, error) {
	if root.CfName() == appName {
		return root, nil
	}

	appsDir := root.Spec.Source.Path // check if it's not in this repo
	filenames, err := filepath.Glob(filepath.Join(e.c.path, appsDir, "*.yaml"))
	if err != nil {
		return nil, err
	}

	for _, f := range filenames {
		app, err := getAppFromFile(f, root)
		if err != nil || app == nil {
			// not an argocd app - ignore
			continue
		}

		if !app.IsManagedBy() {
			continue
		}

		res, err := e.getAppByNameRecurse(app, appName)
		if err != nil || res != nil {
			return res, err
		}
	}

	return nil, nil
}

func getAppFromFile(path string, parent *Application) (*Application, error) {
	data, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}

	for _, text := range yamlSeparator.Split(string(data), -1) {
		if strings.TrimSpace(text) == "" {
			continue
		}
		u := &unstructured.Unstructured{}
		err := yaml.Unmarshal([]byte(text), u)
		if err != nil {
			return nil, err
		}

		if u.GetKind() == "Application" {
			app := &v1alpha1.Application{}
			if err := yaml.Unmarshal(data, app); err != nil {
				return nil, err
			}

			return &Application{app, parent, path}, nil
		}
	}

	return nil, nil
}

func (a *Application) SrcPath() string {
	return a.Spec.Source.Path
}

func (a *Application) SetSrcPath(newPath string) {
	a.Spec.Source.Path = newPath
}

func (a *Application) SetPath(newPath string) {
	a.path = newPath
}

func (a *Application) CfName() string {
	return a.labelValue(labelsCfName)
}

func (a *Application) IsManagedBy() bool {
	return a.labelValue(labelsManagedBy) == "codefresh.io"
}

func (a *Application) labelValue(label string) string {
	if a.Labels == nil {
		return ""
	}

	return a.Labels[label]
}

func (a *Application) getBaseLocation(absRoot string) (string, error) {
	refKust := filepath.Join(absRoot, a.SrcPath(), "kustomization.yaml")
	bytes, err := ioutil.ReadFile(refKust)
	if err != nil {
		return "", err
	}

	k := &kustomize.Kustomization{}
	err = yaml.Unmarshal(bytes, k)
	if err != nil {
		return "", err
	}

	return "", nil
}

func (a *Application) Save() error {
	data, err := yaml.Marshal(a)
	if err != nil {
		return err
	}

	return ioutil.WriteFile(a.path, data, 0644)
}
