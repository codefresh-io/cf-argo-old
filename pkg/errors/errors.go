package errors

import (
	"context"

	"github.com/codefresh-io/cf-argo/pkg/log"
)

func MustContext(ctx context.Context, err error) {
	if err != nil {
		log.G(ctx).WithError(err).Fatal("must")
	}
}
