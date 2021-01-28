package install

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/codefresh-io/cf-argo/pkg/errors"
	"github.com/codefresh-io/cf-argo/pkg/git"
	"github.com/codefresh-io/cf-argo/pkg/log"
	"github.com/codefresh-io/cf-argo/pkg/store"
	"github.com/spf13/cobra"
	"sigs.k8s.io/kustomize/api/filesys"
	"sigs.k8s.io/kustomize/api/krusty"
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

	// errors.MustContext(ctx, cmd.MarkFlagRequired("repo-owner"))
	errors.MustContext(ctx, cmd.MarkFlagRequired("repo-name"))
	errors.MustContext(ctx, cmd.MarkFlagRequired("git-token"))

	return cmd
}

func install(ctx context.Context, opts *options) error {
	var err error
	defer func() {
		cleanup(ctx, err != nil, opts)
	}()

	err = cloneBase(ctx, opts.repoName)
	if err != nil {
		return err
	}

	// modify template with local data
	data, err := buildArgocdResources(ctx, filepath.Join(opts.repoName, "argo-cd/overlays/production"))
	if err != nil {
		return err
	}
	fmt.Println(string(data))
	return nil

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

	cloneURL, err := createRepo(ctx, opts)
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

func buildArgocdResources(ctx context.Context, path string) ([]byte, error) {
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

func createRepo(ctx context.Context, opts *options) (string, error) {
	p, err := git.New(&git.Options{
		Type: "github", // TODO: support other types
		Auth: &git.Auth{
			Password: opts.gitToken,
		},
	})
	if err != nil {
		return "", err
	}

	cloneURL, err := p.CreateRepository(ctx, opts.repoOwner, opts.repoName)
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
