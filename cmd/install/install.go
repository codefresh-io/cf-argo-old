package install

import (
	"context"
	"fmt"

	"github.com/codefresh-io/cf-argo/pkg/git"
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
			return install(ctx, opts)
		},
	}

	cmd.Flags().StringVar(&opts.repoOwner, "repo-owner", "", "repo owner")
	cmd.Flags().StringVar(&opts.repoName, "repo-name", "cf-argo", "repo name")
	cmd.Flags().StringVar(&opts.gitToken, "git-token", "", "git token")
	cmd.Flags().StringVar(&opts.kubeContext, "kube-wefwefowemfowenfowejfnwiejfnwiejfnwcontext", "", "kube context")
	cmd.Flags().StringVar(&opts.kubeConfigPath, "kube-config-path", "", "kube context")

	cmd.MarkFlagRequired("repo-owner")
	cmd.MarkFlagRequired("repo-name")
	cmd.MarkFlagRequired("git-token")

	return cmd
}

func install(ctx context.Context, opts options) error {
	baseGitURL := store.Get().BaseGitURL

	o := &git.Options{
		Type: "github",
		Auth: git.Auth{
			Password: opts.gitToken,
		},
	}

	p, err := git.New(o)
	if err != nil {
		return err
	}

	fmt.Printf("cloning %s...\n", baseGitURL)
	_, err = p.CloneRepository(ctx, baseGitURL, "/Users/roikramer/Desktop/test")
	if err != nil {
		return err
	}

	return nil
}
