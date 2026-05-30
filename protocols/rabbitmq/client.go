// Copyright (c) 2026 Mockarty. All rights reserved.
// Licensed under the Mockarty SDK License Agreement. See LICENSE file for details.

package rabbitmq

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"

	"github.com/mockarty/mockarty-go/protocols/telemetry"
)

// Client is a goroutine-safe RabbitMQ test client. One connection +
// one channel are dialed on first use and reused; channel re-creation
// after a transport failure is the caller's responsibility (call
// Close + NewClient).
type Client struct {
	cfg   *config
	url   string
	conn  *amqp.Connection
	chMu  sync.Mutex
	ch    *amqp.Channel
	dialM sync.Mutex
	seq   atomic.Uint64
}

// NewClient builds a RabbitMQ client targeting the supplied AMQP URL
// (e.g. "amqp://guest:guest@localhost:5672/"). The dial is lazy — the
// first Publish / Consume call establishes the connection so test setup
// can mark the client as configured before the broker is up.
func NewClient(url string, opts ...Option) (*Client, error) {
	if url == "" {
		return nil, errors.New("mockarty rabbitmq: empty amqp url")
	}
	cfg := defaultConfig()
	for _, opt := range opts {
		opt(cfg)
	}
	return &Client{cfg: cfg, url: url}, nil
}

// Close releases the channel + connection. Idempotent — second call
// returns the cached error from the first.
func (c *Client) Close() error {
	if c == nil {
		return nil
	}
	c.chMu.Lock()
	defer c.chMu.Unlock()
	var firstErr error
	if c.ch != nil {
		if err := c.ch.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
		c.ch = nil
	}
	if c.conn != nil {
		if err := c.conn.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
		c.conn = nil
	}
	return firstErr
}

// channel returns the cached channel, dialing the connection on first
// use. Goroutine-safe via dialM. Callers MUST NOT close the returned
// channel; lifecycle belongs to Client.Close.
func (c *Client) channel() (*amqp.Channel, error) {
	c.chMu.Lock()
	if c.ch != nil {
		ch := c.ch
		c.chMu.Unlock()
		return ch, nil
	}
	c.chMu.Unlock()
	c.dialM.Lock()
	defer c.dialM.Unlock()
	c.chMu.Lock()
	if c.ch != nil {
		ch := c.ch
		c.chMu.Unlock()
		return ch, nil
	}
	c.chMu.Unlock()
	conn, err := amqp.Dial(c.url)
	if err != nil {
		return nil, fmt.Errorf("mockarty rabbitmq: dial %s: %w", c.url, err)
	}
	ch, err := conn.Channel()
	if err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("mockarty rabbitmq: open channel: %w", err)
	}
	c.chMu.Lock()
	c.conn = conn
	c.ch = ch
	c.chMu.Unlock()
	return ch, nil
}

// Publish sends one message to the exchange/routingKey pair. Payload
// shapes: []byte, string, json.RawMessage, or a value that
// encoding/json can marshal.
//
// Headers map to AMQP message headers; ContentType defaults to
// "application/json" so consumers can rely on it without per-call
// boilerplate.
func (c *Client) Publish(ctx context.Context, exchange, routingKey string, payload any, headers map[string]string) error {
	if c == nil {
		return errors.New("mockarty rabbitmq: nil client")
	}
	stepName := "publish:" + exchange + "/" + routingKey
	// Take the start BEFORE marshalling so a marshal error still
	// records a non-zero duration step (review B7).
	start := time.Now()
	body, err := marshalPayload(payload)
	if err != nil {
		c.recordStep(ctx, stepName, start, time.Since(start), "failed", err, nil)
		return err
	}
	ch, err := c.channel()
	if err != nil {
		c.recordStep(ctx, stepName, start, time.Since(start), "broken", err, nil)
		return err
	}
	pubHeaders := amqp.Table{}
	for k, v := range headers {
		pubHeaders[k] = v
	}
	err = ch.PublishWithContext(ctx, exchange, routingKey, false /*mandatory*/, false /*immediate*/, amqp.Publishing{
		ContentType: "application/json",
		Headers:     pubHeaders,
		Body:        body,
		Timestamp:   time.Now(),
	})
	dur := time.Since(start)
	if err != nil {
		c.recordStep(ctx, stepName, start, dur, classify(err), err, map[string]string{
			"payload":      capPreview(body, c.cfg.payloadCap),
			"payload_size": strconv.Itoa(len(body)),
		})
		return err
	}
	c.recordStep(ctx, stepName, start, dur, "passed", nil, map[string]string{
		"payload":      capPreview(body, c.cfg.payloadCap),
		"payload_size": strconv.Itoa(len(body)),
	})
	return nil
}

// ConsumeOptions controls one Consume call.
type ConsumeOptions struct {
	Queue string
	// MaxMessages caps how many messages to pull before returning.
	// Default 1 — most CI tests assert a single message at a time.
	MaxMessages int
	// AutoAck — when true, server-side ack happens before the
	// message is delivered (lossy but simpler). Default false:
	// each consumed message is explicitly Ack'd before this call
	// returns so test re-runs don't double-consume.
	AutoAck bool
	// Decode, if non-nil, unmarshals the first message Body into
	// the supplied pointer (typically a *map[string]any or struct).
	Decode any
}

// ConsumedMessage carries the bytes the consumer pulled off the queue
// plus the AMQP-side coordinates.
type ConsumedMessage struct {
	Timestamp   time.Time
	Headers     map[string]string
	Exchange    string
	RoutingKey  string
	ContentType string
	Body        []byte
	DeliveryTag uint64
}

// Consume pulls up to opts.MaxMessages messages from the queue via
// Channel.Get (synchronous fetch — not consumer-tag delivery). One
// Step is recorded per Consume call covering all pulled messages, so
// the TCM timeline shows "consume:<queue>" with count + duration.
func (c *Client) Consume(ctx context.Context, opts ConsumeOptions) ([]ConsumedMessage, error) {
	if c == nil {
		return nil, errors.New("mockarty rabbitmq: nil client")
	}
	if opts.Queue == "" {
		return nil, errors.New("mockarty rabbitmq: empty queue")
	}
	if opts.MaxMessages <= 0 {
		opts.MaxMessages = 1
	}
	stepName := "consume:" + opts.Queue
	start := time.Now()
	// Defer the recordStep so a Decode failure (or anything else
	// after the consume loop) can downgrade the verdict from passed
	// → failed without emitting a stale row first (review B5).
	stepStatus := "passed"
	var stepErr error
	out := make([]ConsumedMessage, 0, opts.MaxMessages)
	defer func() {
		c.recordStep(ctx, stepName, start, time.Since(start), stepStatus, stepErr, map[string]string{
			"count": strconv.Itoa(len(out)),
		})
	}()
	ch, err := c.channel()
	if err != nil {
		stepStatus = "broken"
		stepErr = err
		return nil, err
	}
	for i := 0; i < opts.MaxMessages; i++ {
		if err := ctx.Err(); err != nil {
			stepStatus = "broken"
			stepErr = err
			return out, err
		}
		msg, ok, gErr := ch.Get(opts.Queue, opts.AutoAck)
		if gErr != nil {
			stepStatus = classify(gErr)
			stepErr = gErr
			return out, gErr
		}
		if !ok {
			break
		}
		out = append(out, toConsumedMessage(msg))
		if !opts.AutoAck {
			if err := msg.Ack(false); err != nil {
				stepStatus = "broken"
				stepErr = err
				return out, fmt.Errorf("mockarty rabbitmq: ack: %w", err)
			}
		}
	}
	if opts.Decode != nil && len(out) > 0 {
		if err := json.Unmarshal(out[0].Body, opts.Decode); err != nil {
			stepStatus = "failed"
			stepErr = err
			return out, fmt.Errorf("mockarty rabbitmq: decode first message into %T: %w", opts.Decode, err)
		}
	}
	return out, nil
}

// DeclareQueueOptions wraps amqp.QueueDeclare flags.
type DeclareQueueOptions struct {
	Args       map[string]any
	Durable    bool
	AutoDelete bool
	Exclusive  bool
	NoWait     bool
}

// DeclareQueue idempotently declares a queue. Useful for test setup —
// declares are cheap and the server returns OK when the queue already
// matches.
func (c *Client) DeclareQueue(ctx context.Context, name string, opts DeclareQueueOptions) error {
	if c == nil {
		return errors.New("mockarty rabbitmq: nil client")
	}
	if name == "" {
		return errors.New("mockarty rabbitmq: empty queue name")
	}
	start := time.Now()
	ch, err := c.channel()
	if err != nil {
		c.recordStep(ctx, "declare-queue:"+name, start, time.Since(start), "broken", err, nil)
		return err
	}
	args := amqp.Table{}
	for k, v := range opts.Args {
		args[k] = v
	}
	_, err = ch.QueueDeclare(name, opts.Durable, opts.AutoDelete, opts.Exclusive, opts.NoWait, args)
	dur := time.Since(start)
	if err != nil {
		c.recordStep(ctx, "declare-queue:"+name, start, dur, classify(err), err, nil)
		return err
	}
	c.recordStep(ctx, "declare-queue:"+name, start, dur, "passed", nil, nil)
	return nil
}

func (c *Client) recordStep(ctx context.Context, name string, start time.Time, dur time.Duration, status string, err error, params map[string]string) {
	seq := c.seq.Add(1)
	step := telemetry.Step{
		StartedAt:  start,
		EndedAt:    start.Add(dur),
		Parameters: params,
		Key:        telemetry.NewStepKey(name, seq),
		Name:       name,
		Status:     status,
		Duration:   dur,
	}
	if err != nil {
		step.Message = err.Error()
	}
	c.cfg.recorder.Record(ctx, step)
}

func toConsumedMessage(m amqp.Delivery) ConsumedMessage {
	out := ConsumedMessage{
		Timestamp:   m.Timestamp,
		Exchange:    m.Exchange,
		RoutingKey:  m.RoutingKey,
		ContentType: m.ContentType,
		Body:        append([]byte(nil), m.Body...),
		DeliveryTag: m.DeliveryTag,
	}
	if len(m.Headers) > 0 {
		out.Headers = make(map[string]string, len(m.Headers))
		for k, v := range m.Headers {
			out.Headers[k] = fmt.Sprintf("%v", v)
		}
	}
	return out
}

func marshalPayload(p any) ([]byte, error) {
	switch v := p.(type) {
	case nil:
		return nil, nil
	case []byte:
		return v, nil
	case string:
		return []byte(v), nil
	case json.RawMessage:
		return v, nil
	}
	b, err := json.Marshal(p)
	if err != nil {
		return nil, fmt.Errorf("mockarty rabbitmq: marshal payload: %w", err)
	}
	return b, nil
}

// classify maps amqp errors to a telemetry step status. Connection
// resets, ctx cancellation, and i/o errors are "broken" (env failure).
// AMQP exception codes (resource-locked, access-refused, …) come
// back as *amqp.Error; we treat them as "failed" (assertion).
func classify(err error) string {
	if err == nil {
		return "passed"
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return "broken"
	}
	var amqpErr *amqp.Error
	if errors.As(err, &amqpErr) {
		return "failed"
	}
	return "broken"
}

// capPreview is a thin alias to telemetry.CapPreview — UTF-8 rune
// boundary handling + truncation marker live in the shared helper.
func capPreview(body []byte, cap int) string {
	return telemetry.CapPreview(body, cap)
}
