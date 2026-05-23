// Copyright (c) 2026 Mockarty. All rights reserved.
// Licensed under the Mockarty Software License Agreement.
// See LICENSE file in the project root for full license text.

package mockartycontainer

import (
	"context"
	"fmt"
	"net/http"
	"time"
)

// WaitReady polls the container's /health endpoint until it returns
// 200, the supplied context is cancelled, or the deadline elapses.
//
// New / Run already wait for /health to come up before they return, so
// most callers do not need this. It is here for the rare case where
// the caller pauses the container (docker pause) and wants to assert
// it has been unpaused before sending traffic again.
//
// Polls every 250ms by default; pass WithReadyInterval to override.
func (m *MockartyContainer) WaitReady(ctx context.Context, opts ...ReadyOption) error {
	cfg := readyConfig{interval: 250 * time.Millisecond, timeout: 30 * time.Second}
	for _, o := range opts {
		o(&cfg)
	}
	deadline := time.Now().Add(cfg.timeout)
	url := m.MetricsURL() + "/health"
	for {
		if time.Now().After(deadline) {
			return fmt.Errorf("mockartycontainer: WaitReady deadline (%s) exceeded", cfg.timeout)
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err == nil {
			resp, err := m.httpClient.Do(req)
			if err == nil {
				_ = resp.Body.Close()
				if resp.StatusCode == http.StatusOK {
					return nil
				}
			}
		}
		select {
		case <-time.After(cfg.interval):
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

// readyConfig knobs for WaitReady.
type readyConfig struct {
	interval time.Duration
	timeout  time.Duration
}

// ReadyOption mutates a WaitReady invocation.
type ReadyOption func(*readyConfig)

// WithReadyInterval overrides the poll cadence (default 250ms).
func WithReadyInterval(d time.Duration) ReadyOption {
	return func(c *readyConfig) {
		if d > 0 {
			c.interval = d
		}
	}
}

// WithReadyTimeout overrides the overall deadline (default 30s).
func WithReadyTimeout(d time.Duration) ReadyOption {
	return func(c *readyConfig) {
		if d > 0 {
			c.timeout = d
		}
	}
}
