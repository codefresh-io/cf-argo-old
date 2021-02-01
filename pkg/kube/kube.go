package kube

import (
	"context"
	"path/filepath"
	"time"

	"github.com/codefresh-io/cf-argo/pkg/errors"
	cferrors "github.com/codefresh-io/cf-argo/pkg/errors"
	"github.com/codefresh-io/cf-argo/pkg/log"

	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/util/homedir"
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
		IOStreams genericclioptions.IOStreams

		// Overwrite automatically resolve conflicts between the modified and live configuration by using values from
		// the modified configuration
		Overwrite bool

		// DryRunStrategy by default false, can be set to either "client" or "server" dry-run modes, see kubectl apply --help
		DryRunStrategy kcmdutil.DryRunStrategy
	}
)

func NewConfig() *Config {
	return &Config{genericclioptions.NewConfigFlags(true)}
}

func (c *Config) FlagSet(ctx context.Context) *pflag.FlagSet {
	flags := pflag.NewFlagSet("kubernetes", pflag.ContinueOnError)

	errors.MustContext(ctx, viper.BindEnv("kubeconfig", "KUBECONFIG"))
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
	var l log.Logger
	if *cfg.cfg.Context != "" {
		l = l.WithField("context", *cfg.cfg.Context)
	}

	return &Client{kcmdutil.NewFactory(kcmdutil.NewMatchVersionFlags(cfg.cfg)), l}
}

func Apply(ctx context.Context, opts *ApplyOptions) error {
	if opts == nil {
		return cferrors.ErrNilOpts
	}
}

func defaultConfigPath() string {
	if home := homedir.HomeDir(); home != "" {
		return filepath.Join(home, ".kube", "config")
	}
	return ""
}
