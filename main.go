package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/codefresh-io/cf-argo/cmd/root"
)

func main() {
	ctx := context.Background()
	ctx = contextWithCancel(ctx)

	c := root.New(ctx)
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
