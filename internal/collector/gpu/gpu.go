package gpu

import (
	"context"

	"github.com/mesutoezdil/tensorwatch/internal/model"
)

type Collector interface {
	Name() string
	Collect(ctx context.Context, into *model.Snapshot) error
	Close() error
	Available() bool
}

type noop struct{}

func (noop) Name() string                                 { return "gpu-noop" }
func (noop) Collect(_ context.Context, _ *model.Snapshot) error { return nil }
func (noop) Close() error                                 { return nil }
func (noop) Available() bool                              { return false }

func Noop() Collector { return noop{} }
