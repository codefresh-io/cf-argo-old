package kube

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"time"

	cferrors "github.com/codefresh-io/cf-argo/pkg/errors"
	"github.com/codefresh-io/cf-argo/pkg/log"

	fakeio "github.com/rhysd/go-fakeio"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/util/homedir"
	"k8s.io/kubectl/pkg/cmd/apply"
	kcmdutil "k8s.io/kubectl/pkg/cmd/util"
)

const (
	defaultPollInterval = time.Second * 2
	defaultPollTimeout  = time.Second * 5
)

type (
	Config struct {
		cfg *genericclioptions.ConfigFlags
	}

	Client struct {
		kcmdutil.Factory
		log log.Logger
	}

	// WaitOptions struct {
	// 	Interval  time.Duration
	// 	Timeout   time.Duration
	// 	Resources []runtime.Object
	// }

	ApplyOptions struct {
		// IOStreams the std streams used by the apply command
		Manifests []byte

		// DryRunStrategy by default false, can be set to either "client" or "server" dry-run modes, see kubectl apply --help
		DryRunStrategy kcmdutil.DryRunStrategy
	}
)

func NewConfig() *Config {
	return &Config{genericclioptions.NewConfigFlags(true)}
}

func (c *Config) FlagSet(ctx context.Context) *pflag.FlagSet {
	flags := pflag.NewFlagSet("kubernetes", pflag.ContinueOnError)

	cferrors.MustContext(ctx, viper.BindEnv("kubeconfig", "KUBECONFIG"))
	viper.SetDefault("kubeconfig", defaultConfigPath())

	flags.StringVar(c.cfg.KubeConfig, "kubeconfig", viper.GetString("kubeconfig"), "path to the kubeconfig file [KUBECONFIG]")
	flags.StringVar(c.cfg.Context, "kube-context", viper.GetString("kube-context"), "name of the kubeconfig context to use")

	return flags
}

// func (c *Client) WaitResourceReady(ctx context.Context, opts *WaitOptions) error {
// 	interval := defaultPollInterval
// 	timeout := defaultPollTimeout
// 	resources := make(map[runtime.Object]bool)
// 	i := 0
// 	if opts == nil {
// 		return cferrors.ErrNilOpts
// 	}

// 	if opts.Interval != time.Duration(0) {
// 		interval = opts.Interval
// 	}
// 	if opts.Timeout != time.Duration(0) {
// 		timeout = opts.Timeout
// 	}

// 	for _, r := range opts.Resources {
// 		resources[r] = true // add the resources to the set
// 	}

// 	l := log.G(ctx).WithFields(log.Fields{
// 		"interval": interval,
// 		"timeout":  timeout,
// 		"itr":      i,
// 	})
// 	return wait.PollImmediate(interval, timeout, func() (bool, error) {
// 		l.Debug("starting to check kubernetes resources readiness")
// 		var err error

// 		for o := range resources {
// 			l := l.WithField("resource", o.GetObjectKind().GroupVersionKind())
// 			l.Debug("checking if resource is ready")

// 			ready, err := isReady(o)
// 			if err != nil {
// 				return false, err
// 			}

// 			if ready {
// 				l.Debug("resource is ready")
// 				delete(resources, o)
// 			} else {
// 				l.Debug("resource is not ready")
// 			}
// 		}

// 		return len(resources) == 0, err
// 	})
// }

func NewForConfig(ctx context.Context, cfg *Config) *Client {
	l := log.G(ctx)
	if *cfg.cfg.Context != "" {
		l = l.WithField("context", *cfg.cfg.Context)
	}

	return &Client{kcmdutil.NewFactory(kcmdutil.NewMatchVersionFlags(cfg.cfg)), l}
}

func (c *Client) Apply(ctx context.Context, opts *ApplyOptions) error {
	if opts == nil {
		return cferrors.ErrNilOpts
	}

	if opts.Manifests == nil {
		return errors.New("no manifests")
	}

	applyWithTrack := ""
	applyWithStatus := false
	prune := false
	ios := genericclioptions.IOStreams{
		In:     os.Stdin,
		Out:    os.Stdout,
		ErrOut: os.Stderr,
	}

	o := apply.NewApplyOptions(ios)

	applyCmd := &cobra.Command{
		Use:   "apply",
		Short: "Apply a configuration to a resource in kubernetes",
		RunE: func(cmd *cobra.Command, args []string) error {
			o.DeleteFlags.FileNameFlags.Filenames = &[]string{"-"}
			o.Overwrite = true
			o.Prune = prune
			o.PruneWhitelist = []string{
				"/v1/ConfigMap",
				"/v1/PersistentVolumeClaim",
				"/v1/Secret",
				"/v1/Service",
				"/v1/ServiceAccount",
				"apps/v1/DaemonSet",
				"apps/v1/Deployment",
				"batch/v1beta1/CronJob",
				// "networking/v1/Ingress",
			}
			o.DryRunStrategy = opts.DryRunStrategy

			if o.Namespace != "" {
				o.EnforceNamespace = true
			}

			err := o.Complete(c, cmd)
			if err != nil {
				return err
			}

			fake := fakeio.StdinBytes([]byte{})
			defer fake.Restore()
			go func() {
				fake.StdinBytes(opts.Manifests)
				fake.CloseStdin()
			}()

			return o.Run()
		},
	}

	kcmdutil.AddDryRunFlag(applyCmd)
	kcmdutil.AddServerSideApplyFlags(applyCmd)
	kcmdutil.AddValidateFlags(applyCmd)
	kcmdutil.AddFieldManagerFlagVar(applyCmd, &o.FieldManager, apply.FieldManagerClientSideApply)

	applyCmd.Flags().BoolVar(&prune, "prune", false, "")
	applyCmd.Flags().BoolVar(&applyWithStatus, "status", false, "")
	applyCmd.Flags().StringVar(&applyWithTrack, "track", "ready", "")
	applyCmd.SetArgs([]string{})

	return applyCmd.Execute()
}

func defaultConfigPath() string {
	if home := homedir.HomeDir(); home != "" {
		return filepath.Join(home, ".kube", "config")
	}
	return ""
}
