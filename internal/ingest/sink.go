package ingest

import (
	"context"

	"github.com/cun0/insider-case/internal/domain"
)

type Sink interface {
	Start() error
	Stop(ctx context.Context) error
	Submit(ctx context.Context, e domain.Event) (Result, error)
}
