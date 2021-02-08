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
	"text/template"
	"time"

	envman "github.com/codefresh-io/cf-argo/pkg/environments-manager"
	"github.com/codefresh-io/cf-argo/pkg/errors"
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
	dryRun    bool
}

var values struct {
	ArgoAppsDir           string
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
	viper.SetDefault("env-name", "production")

	cmd.Flags().StringVar(&opts.repoURL, "repo-url", viper.GetString("repo-url"), "the gitops repository url. If it does not exist we will try to create it for you [REPO_URL]")
	cmd.Flags().StringVar(&opts.envName, "env-name", viper.GetString("env-name"), "name of the Argo Enterprise environment to create")
	cmd.Flags().StringVar(&opts.gitToken, "git-token", viper.GetString("git-token"), "git token which will be used by argo-cd to create the gitops repository")
	cmd.Flags().BoolVar(&opts.dryRun, "dry-run", false, "when true, the command will have no side effects, and will only output the manifests to stdout")

	errors.MustContext(ctx, cmd.MarkFlagRequired("repo-url"))
	errors.MustContext(ctx, cmd.MarkFlagRequired("git-token"))

	return cmd
}

// fill the values used to render the templates
func fillValues(opts *options) {
	var err error
	opts.repoOwner, opts.repoName, err = git.SplitCloneURL(opts.repoURL)
	errors.CheckErr(err)

	values.ArgoAppsDir = "argocd-apps"
	values.GitToken = base64.StdEncoding.EncodeToString([]byte(opts.gitToken))
	values.EnvName = opts.envName
	values.Namespace = fmt.Sprintf("%s-argocd", values.EnvName)
	values.RepoOwner = opts.repoOwner
	values.RepoName = opts.repoName
}

func install(ctx context.Context, opts *options) {
	defer func() {
		if err := recover(); err != nil {
			cleanup(ctx, true, opts)
			panic(err)
		}
		cleanup(ctx, false, opts)
	}()

	log.G(ctx).Printf("cloning template repository...")
	prepareBase(ctx, opts.repoName)

	tryCloneGitopsRepo(ctx, opts)

	log.G(ctx).Printf("building bootstrap resources...")
	data := buildBootstrapResources(ctx, opts.repoName)

	if opts.dryRun {
		log.G(ctx).Printf(string(data))
		return
	}

	log.G(ctx).Printf("applying resource to cluster...")
	errors.CheckErr(applyBootstrapResources(ctx, data, opts))

	log.G(ctx).Printf("waiting for argocd initialization to complete... (might take a few seconds)")
	errors.CheckErr(waitForDeployments(ctx, opts))

	errors.CheckErr(createSealedSecret(ctx, opts))

	persistGitopsRepo(ctx, opts)

	errors.CheckErr(createArgocdApp(ctx, opts))

	passwd, err := getArgocdPassword(ctx, opts)
	errors.CheckErr(err)

	log.G(ctx).Printf("\n\nargocd initialized. password: %s", passwd)
	log.G(ctx).Printf("run: kubectl port-forward -n %s svc/argocd-server 8080:80", values.Namespace)
}

func prepareBase(ctx context.Context, path string) {
	var err error
	log.G(ctx).Debug("creating temp dir for template repo")
	values.TemplateRepoClonePath, err = ioutil.TempDir("", "")
	errors.CheckErr(err)

	log.G(ctx).WithField("location", values.TemplateRepoClonePath).Debug("temp dir created")

	_, err = git.Clone(ctx, &git.CloneOptions{
		URL:  store.Get().BaseGitURL,
		Path: values.TemplateRepoClonePath,
	})
	errors.CheckErr(err)

	log.G(ctx).Debug("cleaning template repository")
	errors.CheckErr(os.RemoveAll(filepath.Join(path, ".git")))

	log.G(ctx).Debug("renaming envName files")
	errors.CheckErr(helpers.RenameFilesWithEnvName(ctx, values.TemplateRepoClonePath, values.EnvName))

	log.G(ctx).Debug("rendering template values")
	errors.CheckErr(helpers.RenderDirRecurse(filepath.Join(path, "**/*.*"), values))
}

func tryCloneGitopsRepo(ctx context.Context, opts *options) {
	p, err := git.New(&git.Options{
		Type: "github", // only option for now
		Auth: &git.Auth{
			Password: opts.gitToken,
		},
	})
	errors.CheckErr(err)

	cloneURL, err := p.GetRepository(ctx, &git.GetRepositoryOptions{
		Owner: values.RepoOwner,
		Name:  values.RepoName,
	})
	if err != nil {
		if err != git.ErrRepoNotFound {
			panic(err)
		}

		// use the template repo to init the new repo
		values.GitopsRepoClonePath = values.TemplateRepoClonePath
		return
	}

	log.G(ctx).Debug("creating temp dir for template repo")
	values.GitopsRepoClonePath, err = ioutil.TempDir("", "")
	errors.CheckErr(err)
	log.G(ctx).WithField("location", values.GitopsRepoClonePath).Debug("temp dir created")

	log.G(ctx).Printf("cloning existing gitops repository...")

	values.GitopsRepo, err = p.Clone(ctx, &git.CloneOptions{
		URL:  cloneURL,
		Path: values.GitopsRepoClonePath,
	})
	errors.CheckErr(err)
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
	data, err := ioutil.ReadFile(filepath.Join(
		values.RepoName,
		values.ArgoAppsDir,
		fmt.Sprintf("%s.yaml", values.EnvName),
	))
	if err != nil {
		return err
	}

	return apply(ctx, opts, data)
}

func createSealedSecret(ctx context.Context, opts *options) error {
	secretPath := filepath.Join(values.RepoName, "secret.yaml")
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
			values.RepoName,
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

func applyBootstrapResources(ctx context.Context, manifests []byte, opts *options) error {
	return store.Get().NewKubeClient(ctx).Apply(ctx, &kube.ApplyOptions{
		Manifests: manifests,
		DryRun:    opts.dryRun,
	})
}

func buildBootstrapResources(ctx context.Context, path string) []byte {
	opts := krusty.MakeDefaultOptions()
	opts.DoLegacyResourceSort = true

	k := krusty.MakeKustomizer(filesys.MakeFsOnDisk(), opts)
	res, err := k.Run(path)
	errors.CheckErr(err)

	data, err := res.AsYaml()
	errors.CheckErr(err)

	tpl, err := template.New("").Parse(string(data))
	errors.CheckErr(err)

	buf := bytes.NewBuffer(make([]byte, 0, 4096))

	errors.CheckErr(tpl.Execute(buf, values))

	return buf.Bytes()
}

func persistGitopsRepo(ctx context.Context, opts *options) {
	if values.GitopsRepo == nil {
		initializeNewGitopsRepo(ctx, opts)
	} else {
		addToExistingGitopsRepo(ctx, opts)
	}

	errors.CheckErr(values.GitopsRepo.Add(ctx, "."))

	_, err := values.GitopsRepo.Commit(ctx, fmt.Sprintf("added environment %s", values.EnvName))
	errors.CheckErr(err)

	log.G(ctx).Printf("pushing to gitops repo...")
	err = values.GitopsRepo.Push(ctx, &git.PushOptions{
		Auth: &git.Auth{
			Password: opts.gitToken,
		},
	})
	errors.CheckErr(err)
}

func initializeNewGitopsRepo(ctx context.Context, opts *options) {
	var err error
	values.GitopsRepo, err = git.Init(ctx, values.GitopsRepoClonePath)
	errors.CheckErr(err)

	files, err := filepath.Glob(filepath.Join(values.GitopsRepoClonePath, "*.yaml"))
	errors.CheckErr(err)

	for _, f := range files {
		log.G(ctx).WithField("path", f).Debug("removing file")
		errors.CheckErr(os.Remove(f))
	}

	// create new config
	config := envman.NewConfig(values.GitopsRepoClonePath)
	config.AddEnvironmentP(values.EnvName, envman.Environment{
		RootApplicationPath: filepath.Join(
			values.ArgoAppsDir,
			fmt.Sprintf("%s.yaml", values.EnvName),
		),
	})

	log.G(ctx).Printf("creating gitops repository: %s/%s...", opts.repoOwner, opts.repoName)
	cloneURL, err := createRemoteRepo(ctx, opts)
	errors.CheckErr(err)

	errors.CheckErr(values.GitopsRepo.AddRemote(ctx, "origin", cloneURL))
}

func addToExistingGitopsRepo(ctx context.Context, opts *options) {
	config, err := envman.LoadConfig(values.GitopsRepoClonePath)
	errors.CheckErr(err)

}

func createRemoteRepo(ctx context.Context, opts *options) (string, error) {
	p, err := git.New(&git.Options{
		Type: "github", // TODO: support other types
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

func cleanup(ctx context.Context, failed bool, opts *options) {
	if failed || opts.dryRun {
		log.G(ctx).Debugf("cleaning local user repo: %s", opts.repoName)
		if err := os.RemoveAll(opts.repoName); err != nil && !os.IsNotExist(err) {
			log.G(ctx).WithError(err).Error("failed to clean user local repo")
		}
	}
}
