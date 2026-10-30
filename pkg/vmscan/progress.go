package vmscan

import (
	"context"
	"io"
	"log/slog"
	"sync"
	"time"
)

type scanProgress struct {
	logger       *slog.Logger
	started      time.Time
	mu           sync.Mutex
	phase        string
	phaseStarted time.Time
	stop         chan struct{}
	done         chan struct{}
}

func newScanProgress(ctx context.Context, output io.Writer, interval time.Duration) *scanProgress {
	if output == nil {
		output = io.Discard
	}
	p := &scanProgress{
		logger:  slog.New(slog.NewTextHandler(output, nil)).With("scanner", "vm"),
		started: time.Now(), phaseStarted: time.Now(), phase: "initializing", stop: make(chan struct{}), done: make(chan struct{}),
	}
	p.logger.InfoContext(ctx, "VM scan started")
	go func() {
		defer close(p.done)
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-p.stop:
				return
			case <-ctx.Done():
				return
			case <-ticker.C:
				p.mu.Lock()
				p.logger.InfoContext(ctx, "VM scan running", "phase", p.phase, "elapsed", time.Since(p.started).Round(time.Second), "phase_elapsed", time.Since(p.phaseStarted).Round(time.Second))
				p.mu.Unlock()
			}
		}
	}()
	return p
}

func (p *scanProgress) stage(ctx context.Context, phase string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.phase = phase
	p.phaseStarted = time.Now()
	p.logger.InfoContext(ctx, phase, "elapsed", time.Since(p.started).Round(time.Second))
}

func (p *scanProgress) finish(ctx context.Context, err error) {
	close(p.stop)
	<-p.done
	if err != nil {
		p.logger.ErrorContext(ctx, "VM scan failed", "phase", p.phase, "elapsed", time.Since(p.started).Round(time.Second), "error", err)
		return
	}
	p.logger.InfoContext(ctx, "VM scan completed successfully", "elapsed", time.Since(p.started).Round(time.Second))
}
