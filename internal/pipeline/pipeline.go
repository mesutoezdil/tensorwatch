package pipeline

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/mesutoezdil/tensorwatch/internal/collector"
	"github.com/mesutoezdil/tensorwatch/internal/model"
)

type Decorator func(*model.Snapshot)

type Pipeline struct {
	collectors collector.Set
	interval   time.Duration
	decorators []Decorator

	mu          sync.RWMutex
	subscribers []chan model.Snapshot
	last        *model.Snapshot
}

func New(collectors collector.Set, interval time.Duration, decorators ...Decorator) *Pipeline {
	if interval < 100*time.Millisecond {
		interval = 100 * time.Millisecond
	}
	return &Pipeline{collectors: collectors, interval: interval, decorators: decorators}
}

func (p *Pipeline) Subscribe(buf int) <-chan model.Snapshot {
	if buf < 1 {
		buf = 1
	}
	ch := make(chan model.Snapshot, buf)
	p.mu.Lock()
	p.subscribers = append(p.subscribers, ch)
	p.mu.Unlock()
	return ch
}

func (p *Pipeline) Latest() *model.Snapshot {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if p.last == nil {
		return nil
	}
	cp := *p.last
	return &cp
}

func (p *Pipeline) Run(ctx context.Context) error {
	t := time.NewTicker(p.interval)
	defer t.Stop()

	p.tick(ctx)
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-t.C:
			p.tick(ctx)
		}
	}
}

func (p *Pipeline) tick(ctx context.Context) {
	snap := &model.Snapshot{Taken: time.Now()}
	var errs []error
	for _, c := range p.collectors {
		if err := c.Collect(ctx, snap); err != nil {
			errs = append(errs, err)
			snap.Warnings = append(snap.Warnings, c.Name()+": "+err.Error())
		}
	}
	if len(errs) == len(p.collectors) && len(p.collectors) > 0 {
		_ = errors.Join(errs...)
	}
	for _, d := range p.decorators {
		d(snap)
	}

	p.mu.Lock()
	p.last = snap
	subs := make([]chan model.Snapshot, len(p.subscribers))
	copy(subs, p.subscribers)
	p.mu.Unlock()

	for _, ch := range subs {
		select {
		case ch <- *snap:
		default:
		}
	}
}
