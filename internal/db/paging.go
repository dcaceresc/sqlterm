package db

import (
	"context"
	"fmt"
	"sync"
)

// Pager supplies explicit demand for additional blocks during one Run or
// RunInvocation. A Pager is single-use and should be created per execution.
type Pager struct {
	blockSize int

	mu             sync.Mutex
	used           bool
	active         bool
	nextGeneration uint64
	pause          *pagePause
}

// PageToken uniquely identifies one paused block-demand generation. Tokens are
// opaque and are valid only with the Pager that emitted them.
type PageToken struct {
	pager      *Pager
	generation uint64
	set        int
}

type pagePause struct {
	token     PageToken
	requested bool
	demand    chan struct{}
}

type pagerContextKey struct{}

type pageRun struct{ pager *Pager }

// NewPager creates a single-execution pager that pauses after each blockSize
// rows. The caller advances only the currently paused result set with Request.
func NewPager(blockSize int) (*Pager, error) {
	if blockSize < 1 {
		return nil, fmt.Errorf("page block size must be positive")
	}
	return &Pager{blockSize: blockSize}, nil
}

// WithPager opts a Run or RunInvocation into block-demand paging. A nil Pager
// leaves the existing eager behavior unchanged.
func WithPager(ctx context.Context, pager *Pager) context.Context {
	if pager == nil {
		return ctx
	}
	return context.WithValue(ctx, pagerContextKey{}, pager)
}

// Request advances only the exact pause identified by token. It returns false
// for early, repeated, stale, foreign-pager, or zero-token requests.
func (p *Pager) Request(token PageToken) bool {
	if p == nil {
		return false
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if !p.active || p.pause == nil || token.pager != p || p.pause.token != token || p.pause.requested {
		return false
	}
	p.pause.requested = true
	p.pause.demand <- struct{}{}
	return true
}

func beginPaging(ctx context.Context) (*pageRun, error) {
	pager, _ := ctx.Value(pagerContextKey{}).(*Pager)
	if pager == nil {
		return nil, nil
	}
	pager.mu.Lock()
	defer pager.mu.Unlock()
	if pager.used {
		return nil, fmt.Errorf("pager is already bound to an execution")
	}
	pager.used = true
	pager.active = true
	return &pageRun{pager: pager}, nil
}

func (r *pageRun) finish() {
	if r == nil {
		return
	}
	r.pager.mu.Lock()
	r.pager.active = false
	r.pager.pause = nil
	r.pager.mu.Unlock()
}

func (r *pageRun) blockSize() int {
	if r == nil {
		return 0
	}
	return r.pager.blockSize
}

func (r *pageRun) wait(ctx context.Context, set, rows int, emit func(Event)) error {
	r.pager.mu.Lock()
	if !r.pager.active {
		r.pager.mu.Unlock()
		return context.Canceled
	}
	r.pager.nextGeneration++
	token := PageToken{pager: r.pager, generation: r.pager.nextGeneration, set: set}
	pause := &pagePause{token: token, demand: make(chan struct{}, 1)}
	r.pager.pause = pause
	r.pager.mu.Unlock()

	emit(Event{Kind: "paused", Set: set, Rows: rows, Token: token})
	select {
	case <-ctx.Done():
	case <-pause.demand:
	}

	r.pager.mu.Lock()
	if r.pager.pause == pause {
		r.pager.pause = nil
	}
	r.pager.mu.Unlock()
	return ctx.Err()
}
