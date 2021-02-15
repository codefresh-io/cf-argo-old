package uninstall

import (
	"context"

	envman "github.com/codefresh-io/cf-argo/pkg/environments-manager"
	cferrors "github.com/codefresh-io/cf-argo/pkg/errors"
	"github.com/codefresh-io/cf-argo/pkg/git"
	"github.com/codefresh-io/cf-argo/pkg/kube"
	"github.com/codefresh-io/cf-argo/pkg/store"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

type options struct {
	repoURL  string
	envName  string
	gitToken string
}

var values struct {
	ArgoAppsDir         string
	RepoName            string
	RepoOwner           string
	GitopsRepoClonePath string
	GitopsRepo          git.Repository
}

func New(ctx context.Context) *cobra.Command {
	var opts options

	cmd := &cobra.Command{
		Use:   "uninstall",
		Short: "uninstalls an Argo Enterprise solution from a specified cluster and installation",
		Long:  "this command will clear all Argo-CD managed resources relating to a specific installation, from a specific cluster",
		Run: func(cmd *cobra.Command, args []string) {
			fillValues(&opts)
			uninstall(ctx, &opts)
		},
	}

	// add kubernetes flags
	store.Get().KubeConfig.AddFlagSet(cmd)

	_ = viper.BindEnv("repo-url", "REPO_URL")
	_ = viper.BindEnv("env-name", "ENV_NAME")
	_ = viper.BindEnv("git-token", "GIT_TOKEN")

	cmd.Flags().StringVar(&opts.repoURL, "repo-url", viper.GetString("repo-url"), "the gitops repository url. If it does not exist we will try to create it for you [REPO_URL]")
	cmd.Flags().StringVar(&opts.envName, "env-name", viper.GetString("env-name"), "name of the Argo Enterprise environment to create")
	cmd.Flags().StringVar(&opts.gitToken, "git-token", viper.GetString("git-token"), "git token which will be used by argo-cd to create the gitops repository")

	cferrors.MustContext(ctx, cmd.MarkFlagRequired("repo-url"))
	cferrors.MustContext(ctx, cmd.MarkFlagRequired("env-name"))

	return cmd
}

// fill the values used to render the templates
func fillValues(opts *options) {
	var err error
	values.RepoOwner, values.RepoName, err = git.SplitCloneURL(opts.repoURL)
	cferrors.CheckErr(err)
}

func uninstall(ctx context.Context, opts *options) {
	var err error
	values.GitopsRepo, err = git.CloneExistingRepo(ctx, values.RepoOwner, values.RepoName, opts.gitToken)
	cferrors.CheckErr(err)

	values.GitopsRepoClonePath, err = values.GitopsRepo.Root()
	cferrors.CheckErr(err)

	_, err = envman.LoadConfig(values.GitopsRepoClonePath)
	cferrors.CheckErr(err)

}

func delete(ctx context.Context, opts *options, filename string) error {
	return store.Get().NewKubeClient(ctx).Delete(ctx, &kube.DeleteOptions{
		FileName: filename,
		DryRun:   false,
	})
}
