package vmscan

import (
	"context"
	"fmt"
	"net/http/httptrace"
	"time"

	"github.com/accuknox/omniscan/pkg/publisher"
	"github.com/accuknox/omniscan/pkg/scanner"
)

type observedPublisher struct {
	next     publisher.Publisher
	progress *scanProgress
	timeout  time.Duration
	remote   bool
}

func (p observedPublisher) Publish(ctx context.Context, findings ...scanner.Findings) error {
	started := time.Now()
	if !p.remote {
		p.progress.stage(ctx, "writing findings to local output directory")
		if err := p.next.Publish(ctx, findings...); err != nil {
			return err
		}
		p.progress.stage(ctx, "local findings written successfully")
		return nil
	}
	ctx, cancel := context.WithTimeout(ctx, p.timeout)
	defer cancel()
	p.progress.stage(ctx, "uploading findings to artifact API")
	p.progress.logger.InfoContext(ctx, "artifact upload started", "timeout", p.timeout)
	trace := &httptrace.ClientTrace{
		GotConn: func(httptrace.GotConnInfo) {
			p.progress.stage(ctx, "sending report to artifact API")
		},
		WroteRequest: func(info httptrace.WroteRequestInfo) {
			if info.Err != nil {
				p.progress.logger.WarnContext(ctx, "artifact upload request interrupted", "error", info.Err)
				return
			}
			p.progress.stage(ctx, "report sent; waiting for artifact API response")
		},
		GotFirstResponseByte: func() {
			p.progress.logger.InfoContext(ctx, "artifact API response started", "upload_elapsed", time.Since(started).Round(time.Second))
		},
	}
	err := p.next.Publish(httptrace.WithClientTrace(ctx, trace), findings...)
	if err != nil {
		if ctx.Err() != nil {
			err = fmt.Errorf("artifact upload stopped after %s (limit %s): %w", time.Since(started).Round(time.Second), p.timeout, ctx.Err())
		}
		p.progress.logger.ErrorContext(ctx, "artifact upload failed", "upload_elapsed", time.Since(started).Round(time.Second), "error", err)
		return err
	}
	p.progress.stage(ctx, "findings uploaded successfully")
	p.progress.logger.InfoContext(ctx, "artifact API accepted findings", "upload_elapsed", time.Since(started).Round(time.Second))
	return nil
}
