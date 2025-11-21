package providers

import (
	"context"

	"github.com/vukan322/devmetrics/internal/core"
)

type Provider interface {
	Name() string
	Fetch(ctx context.Context, handle string) (core.DevStats, error)
}
