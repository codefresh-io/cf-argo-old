package install

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"text/template"

	"github.com/codefresh-io/cf-argo/pkg/errors"
	"github.com/codefresh-io/cf-argo/pkg/git"
	"github.com/codefresh-io/cf-argo/pkg/helpers"
	"github.com/codefresh-io/cf-argo/pkg/log"
	"github.com/codefresh-io/cf-argo/pkg/store"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"sigs.k8s.io/kustomize/api/filesys"
	"sigs.k8s.io/kustomize/api/krusty"
)

type options struct {
	repoOwner string
	repoName  string
	gitToken  string
	dryRun    bool
}

var values struct {
	GitToken  string
	EnvName   string
	RepoOwner string
	RepoName  string
}

func New(ctx context.Context) *cobra.Command {
	var opts options

	cmd := &cobra.Command{
		Use:   "install",
		Short: "installs the Argo Enterprise solution on a specified cluster",
		Long:  `This command will create a new git repository that manages an Argo Enterprise solution using Argo-CD with gitops.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			fillValues(&opts)
			return install(ctx, &opts)
		},
	}

	errors.MustContext(ctx, viper.BindEnv("repo-owner", "REPO_OWNER"))
	errors.MustContext(ctx, viper.BindEnv("repo-name", "REPO_NAME"))
	errors.MustContext(ctx, viper.BindEnv("git-token", "GIT_TOKEN"))
	viper.SetDefault("repo-name", "cf-argo")

	// add kubernetes flags
	s := store.Get()
	cmd.Flags().AddFlagSet(s.KubeConfig.FlagSet(ctx))

	cmd.Flags().StringVar(&opts.repoOwner, "repo-owner", viper.GetString("repo-owner"), "name of the repository owner, defaults to [REPO_OWNER] environment variable")
	cmd.Flags().StringVar(&opts.repoName, "repo-name", viper.GetString("repo-name"), "name of the repository that will be created and used for the bootstrap installation")
	cmd.Flags().StringVar(&opts.gitToken, "git-token", viper.GetString("git-token"), "git token which will be used by argo-cd to create the gitops repository")
	cmd.Flags().BoolVar(&opts.dryRun, "dry-run", false, "when true, the command will have no side effects, and will only output the manifests to stdout")

	errors.MustContext(ctx, cmd.MarkFlagRequired("repo-owner"))
	errors.MustContext(ctx, cmd.MarkFlagRequired("git-token"))

	return cmd
}

// fill the values used to render the templates
func fillValues(opts *options) {
	values.GitToken = base64.StdEncoding.EncodeToString([]byte(opts.gitToken))
	values.EnvName = "production"
	values.RepoOwner = opts.repoOwner
	values.RepoName = opts.repoName
}

func install(ctx context.Context, opts *options) error {
	var err error
	defer func() {
		cleanup(ctx, err != nil, opts)
	}()

	// createSealedSecret(ctx)
	err = cloneBase(ctx, opts.repoName)
	if err != nil {
		return err
	}

	// modify template with local data
	data, err := buildBootstrapResources(ctx, opts.repoName)
	if err != nil {
		return err
	}

	out, err := renderTpl(string(data), values)
	if err != nil {
		return err
	}
	fmt.Println(out)

	if opts.dryRun {
		return nil
	}

	err = helpers.RenderDirRecurse(filepath.Join(opts.repoName, "**/*.yaml"), values)
	if err != nil {
		return err
	}

	err = persistGitopsRepo(ctx, opts)
	if err != nil {
		return err
	}

	return nil
}

func renderTpl(tpls string, values interface{}) ([]byte, error) {
	tpl, err := template.New("").Parse(tpls)
	if err != nil {
		return nil, err
	}

	buf := bytes.NewBuffer(make([]byte, 0, 4096))

	err = tpl.Execute(buf, values)
	if err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

func buildBootstrapResources(ctx context.Context, path string) ([]byte, error) {
	k := krusty.MakeKustomizer(filesys.MakeFsOnDisk(), krusty.MakeDefaultOptions())
	res, err := k.Run(path)
	if err != nil {
		return nil, err
	}

	data, err := res.AsYaml()
	if err != nil {
		return nil, err
	}

	return data, nil
}

func persistGitopsRepo(ctx context.Context, opts *options) error {
	r, err := git.Init(ctx, opts.repoName)
	if err != nil {
		return err
	}

	err = r.Add(ctx, ".")
	if err != nil {
		return err
	}

	_, err = r.Commit(ctx, "Initial commit")
	if err != nil {
		return err
	}

	cloneURL, err := createRemoteRepo(ctx, opts)
	if err != nil {
		return err
	}

	err = r.AddRemote(ctx, "origin", cloneURL)
	if err != nil {
		return err
	}

	err = r.Push(ctx, &git.PushOptions{
		Auth: &git.Auth{
			Password: opts.gitToken,
		},
	})
	if err != nil {
		return err
	}

	return nil
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

func cloneBase(ctx context.Context, path string) error {
	baseGitURL := store.Get().BaseGitURL
	_, err := git.Clone(ctx, &git.CloneOptions{
		URL:  baseGitURL,
		Path: path,
	})
	if err != nil {
		return err
	}

	err = os.RemoveAll(filepath.Join(path, ".git"))
	if err != nil {
		return err
	}

	return err
}

func cleanup(ctx context.Context, failed bool, opts *options) {
	if failed {
		log.G(ctx).Debugf("cleaning local user repo: %s", opts.repoName)
		if err := os.RemoveAll(opts.repoName); err != nil && !os.IsNotExist(err) {
			log.G(ctx).WithError(err).Error("failed to clean user local repo")
		}
	}
}
