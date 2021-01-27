package install

import (
	"context"
	"io/ioutil"
	"os"

	"github.com/codefresh-io/cf-argo/pkg/errors"
	"github.com/codefresh-io/cf-argo/pkg/git"
	"github.com/codefresh-io/cf-argo/pkg/log"
	"github.com/codefresh-io/cf-argo/pkg/store"
	"github.com/spf13/cobra"
)

type options struct {
	repoOwner      string
	repoName       string
	gitToken       string
	kubeContext    string
	kubeConfigPath string
}

func New(ctx context.Context) *cobra.Command {
	var opts options

	cmd := &cobra.Command{
		Use:   "install",
		Short: "installs the argo enterprise solution on a specified cluster",
		RunE: func(cmd *cobra.Command, args []string) error {
			return install(ctx, &opts)
		},
	}

	cmd.Flags().StringVar(&opts.repoOwner, "repo-owner", "", "repo owner")
	cmd.Flags().StringVar(&opts.repoName, "repo-name", "cf-argo", "repo name")
	cmd.Flags().StringVar(&opts.gitToken, "git-token", "", "git token")
	cmd.Flags().StringVar(&opts.kubeContext, "kube-context", "", "kube context")
	cmd.Flags().StringVar(&opts.kubeConfigPath, "kube-config-path", "", "kube context")

	errors.MustContext(ctx, cmd.MarkFlagRequired("repo-owner"))
	errors.MustContext(ctx, cmd.MarkFlagRequired("repo-name"))
	errors.MustContext(ctx, cmd.MarkFlagRequired("git-token"))

	return cmd
}

func install(ctx context.Context, opts *options) error {
	var err error
	tmp, err := ioutil.TempDir("", "")
	if err != nil {
		return err
	}
	defer cleanup(ctx, err != nil, tmp, opts)

	o := &git.Options{
		Type: "github", // TODO: support other types
		Auth: &git.Auth{
			Password: opts.gitToken,
		},
	}

	err = cloneBase(ctx, tmp)
	if err != nil {
		return err
	}

	p, err := git.New(o)
	if err != nil {
		return err
	}

	_, err = createRepo(ctx, p, opts.repoOwner, opts.repoName)
	if err != nil {
		return err
	}

	return nil
}

func createRepo(ctx context.Context, p git.Provider, repoOwner, repoName string) (git.Repository, error) {
	cloneURL, err := p.CreateRepository(ctx, repoOwner, repoName)
	if err != nil {
		return nil, err
	}
	return p.Clone(ctx, &git.CloneOptions{
		Url:  cloneURL,
		Path: repoName,
	})
}

func cloneBase(ctx context.Context, path string) error {
	baseGitURL := store.Get().BaseGitURL
	_, err := git.Clone(ctx, &git.CloneOptions{
		Url:  baseGitURL,
		Path: path,
	})
	if err != nil {
		return err
	}

	return err
}

func cleanup(ctx context.Context, failed bool, tmpdir string, opts *options) {
	log.G(ctx).Debugf("cleaning temp directory: %s", tmpdir)
	if err := os.RemoveAll(tmpdir); err != nil {
		log.G(ctx).WithError(err).Error("failed to clean temp directory")
	}

	if failed {
		log.G(ctx).Debugf("cleaning local user repo: %s", opts.repoName)
		if err := os.RemoveAll(opts.repoName); err != nil && !os.IsNotExist(err) {
			log.G(ctx).WithError(err).Error("failed to clean user local repo")
		}
	}
}
