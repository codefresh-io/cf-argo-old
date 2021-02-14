package install

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"text/template"
	"time"

	envman "github.com/codefresh-io/cf-argo/pkg/environments-manager"
	cferrors "github.com/codefresh-io/cf-argo/pkg/errors"
	"github.com/codefresh-io/cf-argo/pkg/git"
	"github.com/codefresh-io/cf-argo/pkg/helpers"
	"github.com/codefresh-io/cf-argo/pkg/kube"
	"github.com/codefresh-io/cf-argo/pkg/log"
	ss "github.com/codefresh-io/cf-argo/pkg/sealed-secrets"
	"github.com/codefresh-io/cf-argo/pkg/store"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/kustomize/api/filesys"
	"sigs.k8s.io/kustomize/api/krusty"
)

type options struct {
	repoURL   string
	repoOwner string
	repoName  string
	envName   string
	gitToken  string
	baseRepo  string
	dryRun    bool
}

var values struct {
	BootstrapDir          string
	GitToken              string
	EnvName               string
	Namespace             string
	RepoOwner             string
	RepoName              string
	TemplateRepoClonePath string
	GitopsRepoClonePath   string
	GitopsRepo            git.Repository
}

func New(ctx context.Context) *cobra.Command {
	var opts options

	cmd := &cobra.Command{
		Use:   "install",
		Short: "installs the Argo Enterprise solution on a specified cluster",
		Long:  `This command will create a new git repository that manages an Argo Enterprise solution using Argo-CD with gitops.`,
		Run: func(cmd *cobra.Command, args []string) {
			fillValues(&opts)
			install(ctx, &opts)
		},
	}

	// add kubernetes flags
	store.Get().KubeConfig.AddFlagSet(cmd)

	_ = viper.BindEnv("repo-url", "REPO_URL")
	_ = viper.BindEnv("env-name", "ENV_NAME")
	_ = viper.BindEnv("git-token", "GIT_TOKEN")
	_ = viper.BindEnv("base-repo", "BASE_REPO")
	viper.SetDefault("env-name", "production")
	viper.SetDefault("base-repo", store.Get().BaseGitURL)
	viper.SetDefault("dry-run", false)

	cmd.Flags().StringVar(&opts.repoURL, "repo-url", viper.GetString("repo-url"), "the gitops repository url. If it does not exist we will try to create it for you [REPO_URL]")
	cmd.Flags().StringVar(&opts.envName, "env-name", viper.GetString("env-name"), "name of the Argo Enterprise environment to create")
	cmd.Flags().StringVar(&opts.gitToken, "git-token", viper.GetString("git-token"), "git token which will be used by argo-cd to create the gitops repository")
	cmd.Flags().BoolVar(&opts.dryRun, "dry-run", viper.GetBool("dry-run"), "when true, the command will have no side effects, and will only output the manifests to stdout")
	cmd.Flags().StringVar(&opts.baseRepo, "base-repo", viper.GetString("base-repo"), "the template repository url")

	cferrors.MustContext(ctx, cmd.MarkFlagRequired("repo-url"))
	cferrors.MustContext(ctx, cmd.MarkFlagRequired("git-token"))
	cferrors.MustContext(ctx, cmd.Flags().MarkHidden("base-repo"))

	return cmd
}

// fill the values used to render the templates
func fillValues(opts *options) {
	var err error
	opts.repoOwner, opts.repoName, err = git.SplitCloneURL(opts.repoURL)
	cferrors.CheckErr(err)

	values.BootstrapDir = "bootstrap"
	values.GitToken = base64.StdEncoding.EncodeToString([]byte(opts.gitToken))
	values.EnvName = opts.envName
	values.Namespace = fmt.Sprintf("%s-argocd", values.EnvName)
	values.RepoOwner = opts.repoOwner
	values.RepoName = opts.repoName
}

func install(ctx context.Context, opts *options) {
	defer func() {
		if err := recover(); err != nil {
			cleanup(ctx, true)
			panic(err)
		}
		cleanup(ctx, false)
	}()

	tryCloneExistingRepo(ctx, opts)

	log.G(ctx).Printf("cloning template repository...")
	prepareBase(ctx, opts)

	log.G(ctx).Printf("building bootstrap resources...")
	installBootstrapResources(ctx, opts)
	if opts.dryRun {
		return
	}

	log.G(ctx).Printf("waiting for argocd initialization to complete... (might take a few seconds)")
	cferrors.CheckErr(waitForDeployments(ctx, opts))

	cferrors.CheckErr(createSealedSecret(ctx, opts))

	persistGitopsRepo(ctx, opts)

	cferrors.CheckErr(createArgocdApp(ctx, opts))

	passwd, err := getArgocdPassword(ctx, opts)
	cferrors.CheckErr(err)

	log.G(ctx).Printf("\n\nargocd initialized. password: %s", passwd)
	log.G(ctx).Printf("run: kubectl port-forward -n %s svc/argocd-server 8080:80", values.Namespace)
}

func prepareBase(ctx context.Context, opts *options) {
	var err error
	log.G(ctx).Debug("creating temp dir for template repo")
	values.TemplateRepoClonePath, err = ioutil.TempDir("", "")
	cferrors.CheckErr(err)

	log.G(ctx).WithField("location", values.TemplateRepoClonePath).Debug("temp dir created")

	_, err = git.Clone(ctx, &git.CloneOptions{
		URL:  opts.baseRepo,
		Path: values.TemplateRepoClonePath,
	})
	cferrors.CheckErr(err)

	log.G(ctx).Debug("cleaning template repository")
	cferrors.CheckErr(os.RemoveAll(filepath.Join(values.TemplateRepoClonePath, ".git")))

	log.G(ctx).Debug("renaming envName files")
	cferrors.CheckErr(helpers.RenameFilesWithEnvName(ctx, values.TemplateRepoClonePath, values.EnvName))

	log.G(ctx).Debug("rendering template values")
	cferrors.CheckErr(helpers.RenderDirRecurse(filepath.Join(values.TemplateRepoClonePath, "**/*.*"), values))
}

func tryCloneExistingRepo(ctx context.Context, opts *options) {
	p, err := git.New(&git.Options{
		Type: "github", // only option for now
		Auth: &git.Auth{
			Password: opts.gitToken,
		},
	})
	cferrors.CheckErr(err)

	cloneURL, err := p.GetRepository(ctx, &git.GetRepositoryOptions{
		Owner: values.RepoOwner,
		Name:  values.RepoName,
	})
	if err != nil {
		if err != git.ErrRepoNotFound {
			panic(err)
		}

		return // we will create it later
	}

	log.G(ctx).Debug("creating temp dir for template repo")
	values.GitopsRepoClonePath, err = ioutil.TempDir("", "")
	cferrors.CheckErr(err)
	log.G(ctx).WithField("location", values.GitopsRepoClonePath).Debug("temp dir created")

	log.G(ctx).Printf("cloning existing gitops repository...")

	values.GitopsRepo, err = p.Clone(ctx, &git.CloneOptions{
		URL:  cloneURL,
		Path: values.GitopsRepoClonePath,
	})
	cferrors.CheckErr(err)

	conf, err := envman.LoadConfig(values.GitopsRepoClonePath)
	cferrors.CheckErr(err)

	if _, exists := conf.Environments[values.EnvName]; exists {
		panic(fmt.Errorf("environment with name \"%s\" already exists in target repository", values.EnvName))
	}
}

func apply(ctx context.Context, opts *options, data []byte) error {
	return store.Get().NewKubeClient(ctx).Apply(ctx, &kube.ApplyOptions{
		Manifests: data,
		DryRun:    opts.dryRun,
	})
}

func waitForDeployments(ctx context.Context, opts *options) error {
	deploymentTest := func(ctx context.Context, cs kubernetes.Interface, ns, name string) (bool, error) {
		d, err := cs.AppsV1().Deployments(ns).Get(ctx, name, v1.GetOptions{})
		if err != nil {
			return false, err
		}
		return d.Status.ReadyReplicas >= *d.Spec.Replicas, nil
	}
	ns := values.Namespace
	o := &kube.WaitOptions{
		Interval: time.Second * 2,
		Timeout:  time.Minute * 2,
		Resources: []*kube.ResourceInfo{
			{
				Name:      "argocd-server",
				Namespace: ns,
				Func:      deploymentTest,
			},
			{
				Name:      "sealed-secrets-controller",
				Namespace: ns,
				Func:      deploymentTest,
			},
		},
		DryRun: opts.dryRun,
	}

	return store.Get().NewKubeClient(ctx).Wait(ctx, o)
}

func getArgocdPassword(ctx context.Context, opts *options) (string, error) {
	cs, err := store.Get().NewKubeClient(ctx).KubernetesClientSet()
	if err != nil {
		return "", err
	}
	secret, err := cs.CoreV1().Secrets(values.Namespace).Get(ctx, "argocd-initial-admin-secret", v1.GetOptions{})
	if err != nil {
		return "", err
	}
	passwd, ok := secret.Data["password"]
	if !ok {
		return "", fmt.Errorf("argocd initial password not found")
	}

	return string(passwd), nil
}

func createArgocdApp(ctx context.Context, opts *options) error {
	tplConf, err := envman.LoadConfig(values.GitopsRepoClonePath)
	cferrors.CheckErr(err)
	absArgoAppsDir := filepath.Join(values.GitopsRepoClonePath, filepath.Dir(tplConf.Environments[values.EnvName].RootApplicationPath))

	projData, err := ioutil.ReadFile(filepath.Join(absArgoAppsDir, fmt.Sprintf("%s-project.yaml", values.EnvName)))
	cferrors.CheckErr(err)

	appData, err := ioutil.ReadFile(filepath.Join(absArgoAppsDir, fmt.Sprintf("%s.yaml", values.EnvName)))
	cferrors.CheckErr(err)

	manifests := []byte(fmt.Sprintf("%s\n\n---\n%s", string(projData), string(appData)))

	return apply(ctx, opts, manifests)
}

func createSealedSecret(ctx context.Context, opts *options) error {
	secretPath := filepath.Join(values.TemplateRepoClonePath, values.BootstrapDir, "secret.yaml")
	s, err := ss.CreateSealedSecretFromSecretFile(ctx, values.Namespace, secretPath, opts.dryRun)
	if err != nil {
		return err
	}

	data, err := json.Marshal(s)
	if err != nil {
		return err
	}

	err = apply(ctx, opts, data)
	if err != nil {
		return err
	}

	return ioutil.WriteFile(
		filepath.Join(
			values.TemplateRepoClonePath,
			"kustomize",
			"components",
			"argo-cd",
			"overlays",
			values.EnvName,
			"sealed-secret.json",
		),
		data,
		0644,
	)
}

func installBootstrapResources(ctx context.Context, opts *options) {
	kopts := krusty.MakeDefaultOptions()
	kopts.DoLegacyResourceSort = true

	k := krusty.MakeKustomizer(filesys.MakeFsOnDisk(), kopts)
	res, err := k.Run(filepath.Join(values.TemplateRepoClonePath, values.BootstrapDir))
	cferrors.CheckErr(err)

	data, err := res.AsYaml()
	cferrors.CheckErr(err)

	tpl, err := template.New("").Parse(string(data))
	cferrors.CheckErr(err)

	buf := bytes.NewBuffer(make([]byte, 0, 4096))

	cferrors.CheckErr(tpl.Execute(buf, values))

	manifests := buf.Bytes()

	cferrors.CheckErr(apply(ctx, opts, manifests))
}

func persistGitopsRepo(ctx context.Context, opts *options) {
	if values.GitopsRepo == nil {
		initializeNewGitopsRepo(ctx, opts)
	} else {
		addToExistingGitopsRepo(ctx, opts)
	}

	cferrors.CheckErr(values.GitopsRepo.Add(ctx, "."))

	_, err := values.GitopsRepo.Commit(ctx, fmt.Sprintf("added environment %s", values.EnvName))
	cferrors.CheckErr(err)

	log.G(ctx).Printf("pushing to gitops repo...")
	err = values.GitopsRepo.Push(ctx, &git.PushOptions{
		Auth: &git.Auth{
			Password: opts.gitToken,
		},
	})
	cferrors.CheckErr(err)
}

func initializeNewGitopsRepo(ctx context.Context, opts *options) {
	var err error
	// use the template repo to init the new repo
	values.GitopsRepoClonePath = values.TemplateRepoClonePath
	values.GitopsRepo, err = git.Init(ctx, values.GitopsRepoClonePath)
	cferrors.CheckErr(err)

	cferrors.CheckErr(os.RemoveAll(filepath.Join(values.TemplateRepoClonePath, values.BootstrapDir)))

	log.G(ctx).Printf("creating gitops repository: %s/%s...", opts.repoOwner, opts.repoName)
	cloneURL, err := createRemoteRepo(ctx, opts)
	cferrors.CheckErr(err)

	cferrors.CheckErr(values.GitopsRepo.AddRemote(ctx, "origin", cloneURL))
}

func addToExistingGitopsRepo(ctx context.Context, opts *options) {
	tplConf, err := envman.LoadConfig(values.TemplateRepoClonePath)
	cferrors.CheckErr(err)

	conf, err := envman.LoadConfig(values.GitopsRepoClonePath)
	cferrors.CheckErr(err)

	if len(conf.Environments) == 0 {
		panic(fmt.Errorf("existing repo has no environments in config file"))
	}

	conf.AddEnvironmentP(values.EnvName, tplConf.FirstEnv())

	// tplEnv := tplConf.FirstEnv()

	// // we use the current first env as a reference to how the new env should be added
	// refEnv := conf.FirstEnv()

	// get all of the argocd apps we want to copy to the existing repo
	// la, err := tplEnv.LeafApps()
	// cferrors.CheckErr(err)

	// for _, tplApp := range la {
	// 	refApp, err := refEnv.GetAppByName(tplApp.CfName())
	// 	if err != nil {
	// 		if errors.Is(err, envman.ErrAppNotFound) { // or whatever
	// 			panic(err)
	// 		}

	// 		// we need to separate between move (or copy) the app.yaml file, and move (or copy) the entire folder-structure of the app.source.path
	// 		// and (another option) - just updating the app.source.path string (so - update the object, and saving back to file)
	// 		// so it's all very confusing

	// 		// CopyApp - Copy the app.yaml file from its current location to a different folder
	// 		// UpdateAppPath - Update the path in memory, and save it back to it's current app.yaml file
	// 		// MoveAppResources - Get the folder-structure referenced by the app.source.path field, and copy it to a new destination folder
	// 		//                        also - call UpdateAppPath to update the reference to the new location
	// 		// I know the names are too long. But that can be the start
	// 	}
	// 	cferrors.CheckErr(err)

	// 	refSrcPath := refApp.Spec.Source.Path
	// 	refKust := filepath.Join(values.GitopsRepoClonePath, refSrcPath, "kustomization.yaml")
	// 	bytes, err := ioutil.ReadFile(refKust)
	// 	cferrors.CheckErr(err)

	// 	k := &kustomize.Kustomization{}
	// 	cferrors.CheckErr(yaml.Unmarshal(bytes, k))

	// 	oldRelPath := tplApp.Spec.Source.Path
	// 	src := filepath.Join(values.TemplateRepoClonePath, oldRelPath)

	// 	newRelPath := filepath.Clean(filepath.Join(refSrcPath, k.Resources[0], "../overlays", values.EnvName))
	// 	dst := filepath.Join(values.GitopsRepoClonePath, newRelPath)

	// 	cferrors.CheckErr(os.Rename(src, dst))
	// 	log.G(ctx).Debugf("moving %s to %s", src, dst)

	// 	tplApp.Spec.Source.Path = newRelPath
	// 	cferrors.CheckErr(tplApp.Save())
	// }

	// log.G(ctx).Printf("saving new environment: %s", values.EnvName)
	// cferrors.CheckErr(conf.AddEnvironmentP(values.EnvName, tplEnv))
}

func createRemoteRepo(ctx context.Context, opts *options) (string, error) {
	p, err := git.New(&git.Options{
		Type: "github", // need to support other types
		Auth: &git.Auth{
			Password: opts.gitToken,
		},
	})
	if err != nil {
		return "", err
	}

	cloneURL, err := p.CreateRepository(ctx, &git.CreateRepositoryOptions{
		Owner:   opts.repoOwner,
		Name:    opts.repoName,
		Private: true,
	})
	if err != nil {
		return "", err
	}

	return cloneURL, err
}

func cleanup(ctx context.Context, failed bool) {
	log.G(ctx).Debugf("cleaning dirs: %s", strings.Join([]string{values.GitopsRepoClonePath, values.TemplateRepoClonePath}, ","))
	if err := os.RemoveAll(values.GitopsRepoClonePath); err != nil && !os.IsNotExist(err) {
		log.G(ctx).WithError(err).Error("failed to clean user local repo")
	}
	if err := os.RemoveAll(values.TemplateRepoClonePath); err != nil && !os.IsNotExist(err) {
		log.G(ctx).WithError(err).Error("failed to clean template repo")
	}
}
