package uninstall

import (
	"context"
	"fmt"

	"github.com/codefresh-io/cf-argo/pkg/errors"
	"github.com/codefresh-io/cf-argo/pkg/log"
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
	Namespace   string
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

	errors.MustContext(ctx, viper.BindEnv("env-name", "ENV_NAME"))

	// add kubernetes flags
	s := store.Get()
	cmd.Flags().AddFlagSet(s.KubeConfig.FlagSet(ctx))

	cmd.Flags().StringVar(&opts.envName, "env-name", viper.GetString("env-name"), "name of the Argo Enterprise environment to create")
	cmd.Flags().BoolVar(&opts.dryRun, "dry-run", false, "when true, the command will have no side effects, and will only output the manifests to stdout")

	errors.MustContext(ctx, cmd.MarkFlagRequired("env-name"))

	return cmd
}

// fill the values used to render the templates
func fillValues(opts *options) {
	values.ArgoAppsDir = "argocd-apps"
	values.EnvName = opts.envName
	values.Namespace = fmt.Sprintf("%s-argocd", values.EnvName)
}

func uninstall(ctx context.Context, opts *options) error {
	log.G(ctx).WithField("envName", values.EnvName).Info("Uninstalling")
	return nil
}
