package kube

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/codefresh-io/cf-argo/pkg/errors"
	cferrors "github.com/codefresh-io/cf-argo/pkg/errors"
	"github.com/codefresh-io/cf-argo/pkg/log"

	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
)

const (
	defaultPollInterval = time.Second * 2
	defaultPollTimeout  = time.Second * 5
)

type (
	Config struct {
		ConfigPath string
		Context    string
		InCluster  bool
	}

	Client struct {
		*kubernetes.Clientset
		cfg *rest.Config
	}

	// WaitOptions struct {
	// 	Interval  time.Duration
	// 	Timeout   time.Duration
	// 	Resources []runtime.Object
	// }

	ApplyOptions struct {
		Stdin  io.Reader
		Stdout io.Writer
		Stderr io.Writer
	}
)

func (c *Config) FlagSet(ctx context.Context) *pflag.FlagSet {
	flags := pflag.NewFlagSet("kubernetes", pflag.ContinueOnError)

	errors.MustContext(ctx, viper.BindEnv("kubeconfig", "KUBECONFIG"))
	viper.SetDefault("kubeconfig", defaultConfigPath())

	flags.StringVar(&c.ConfigPath, "kubeconfig", viper.GetString("kubeconfig"), "path to the kubeconfig file [KUBECONFIG]")
	flags.StringVar(&c.Context, "kube-context", viper.GetString("kube-context"), "name of the kubeconfig context to use")
	flags.BoolVar(&c.InCluster, "in-cluster", false, "set to true when running in cluster")

	return flags
}

func (c *Client) Apply()

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

func NewForConfig(ctx context.Context, cfg *Config) (*Client, error) {
	var (
		c   *rest.Config
		err error
	)

	if cfg == nil {
		return nil, cferrors.ErrNilOpts
	}

	if cfg.InCluster {
		log.G(ctx).Debug("using in-cluster config")
		c, err = rest.InClusterConfig()
	} else {
		if _, err = os.Stat(cfg.ConfigPath); err != nil {
			if !os.IsNotExist(err) {
				return nil, err // some other error
			}
			log.G(ctx).Warnf("kubeconfig does not exist in path: '%s', trying in-cluster config", cfg.ConfigPath)
			c, err = rest.InClusterConfig()
		} else {
			c, err = clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
				&clientcmd.ClientConfigLoadingRules{ExplicitPath: cfg.ConfigPath},
				&clientcmd.ConfigOverrides{CurrentContext: cfg.Context},
			).ClientConfig()
		}
	}

	if err != nil {
		return nil, err
	}

	cs, err := kubernetes.NewForConfig(c)
	if err != nil {
		return nil, err
	}

	return &Client{cs, c}, err
}

func (c *Client) ToRESTConfig() (*rest.Config, error) {
	return c.cfg, nil
}

// func (c *Client) ToDiscoveryClient() (discovery.CachedDiscoveryInterface, error) {
// 	return disc, nil
// }

func defaultConfigPath() string {
	if home := homedir.HomeDir(); home != "" {
		return filepath.Join(home, ".kube", "config")
	}
	return ""
}
