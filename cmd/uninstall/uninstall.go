package uninstall

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/codefresh-io/cf-argo/pkg/errors"
	"github.com/codefresh-io/cf-argo/pkg/helpers"
	"github.com/codefresh-io/cf-argo/pkg/kube"
	"github.com/codefresh-io/cf-argo/pkg/store"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

type options struct {
	repoOwner string
	repoName  string
	envName   string
	gitToken  string
	dryRun    bool
}

var values struct {
	ArgoAppsDir string
	EnvName     string
	RepoName    string
}

func New(ctx context.Context) *cobra.Command {
	var opts options

	cmd := &cobra.Command{
		Use:   "uninstall",
		Short: "uninstalls an Argo Enterprise solution from a specified cluster and installation",
		Long:  "this command will clear all Argo-CD managed resources relating to a specific installation, from a specific cluster",
		RunE: func(cmd *cobra.Command, args []string) error {
			fillValues(&opts)
			return uninstall(ctx, &opts)
		},
	}

	errors.MustContext(ctx, viper.BindEnv("repo-name", "REPO_NAME"))
	errors.MustContext(ctx, viper.BindEnv("env-name", "ENV_NAME"))
	viper.SetDefault("repo-name", "cf-argo")

	// add kubernetes flags
	s := store.Get()
	cmd.Flags().AddFlagSet(s.KubeConfig.FlagSet(ctx))

	cmd.Flags().StringVar(&opts.repoName, "repo-name", viper.GetString("repo-name"), "name of the repository that will be created and used for the bootstrap installation")
	cmd.Flags().StringVar(&opts.envName, "env-name", viper.GetString("env-name"), "name of the Argo Enterprise environment to create")
	cmd.Flags().BoolVar(&opts.dryRun, "dry-run", false, "when true, the command will have no side effects, and will only output the manifests to stdout")

	errors.MustContext(ctx, cmd.MarkFlagRequired("env-name"))
	errors.MustContext(ctx, cmd.MarkFlagRequired("repo-name"))

	return cmd
}

// fill the values used to render the templates
func fillValues(opts *options) {
	values.ArgoAppsDir = "argocd-apps"
	values.EnvName = opts.envName
	values.RepoName = opts.repoName
}

func uninstall(ctx context.Context, opts *options) error {
	// rootPath := filepath.Join(values.RepoName, values.ArgoAppsDir, fmt.Sprintf("%s.yaml", values.EnvName))
	// err := delete(ctx, opts, rootPath)
	// return err
	err := clearOverlays(ctx)
	if err != nil {
		return err
	}

	return err
}

func delete(ctx context.Context, opts *options, filename string) error {
	return store.Get().NewKubeClient(ctx).Delete(ctx, &kube.DeleteOptions{
		FileName: filename,
		DryRun:   opts.dryRun,
	})
}

func clearOverlays(ctx context.Context) error {
	pattern := fmt.Sprintf("kustomize/*/*/overlays/%s", values.EnvName)
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return err
	}

	for _, m := range matches {
		if m == fmt.Sprintf("kustomize/components/argo-cd/overlays/%s", values.EnvName) {
			continue
		}

		err = helpers.ClearFolder(ctx, m)
		if err != nil {
			return err
		}
	}

	return nil
}
