package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/codefresh-io/cf-argo/cmd/root"
	"github.com/codefresh-io/cf-argo/pkg/log"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

func main() {
	ctx := context.Background()
	ctx = contextWithCancel(ctx)

	logrusLogger := logrus.StandardLogger()
	ctx = log.WithLogger(ctx, log.FromLogrus(logrus.NewEntry(logrusLogger)))

	logCfg := &log.Config{}

	c := root.New(ctx)
	c.PersistentFlags().AddFlagSet(logCfg.FlagSet())
	c.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
		return log.ConfigureLogrus(logrusLogger, logCfg)
	}

	if err := c.Execute(); err != nil {
		panic(err)
	}
}

func contextWithCancel(ctx context.Context) context.Context {
	ctx, cancel := context.WithCancel(ctx)
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGTERM, syscall.SIGINT)

	go func() {
		<-sig
		cancel()
	}()

	return ctx
}
