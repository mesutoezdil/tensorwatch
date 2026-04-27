package collector

import (
	"context"

	"github.com/mesutoezdil/tensorwatch/internal/model"
)

type Collector interface {
	Name() string
	Collect(ctx context.Context, into *model.Snapshot) error
	Close() error
}

type Set []Collector

func (s Set) Close() {
	for _, c := range s {
		_ = c.Close()
	}
}
