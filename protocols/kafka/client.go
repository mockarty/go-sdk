// Copyright (c) 2026 Mockarty. All rights reserved.
// Licensed under the Mockarty SDK License Agreement. See LICENSE file for details.

package kafka

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/segmentio/kafka-go"

	"github.com/mockarty/mockarty-go/protocols/telemetry"
)

// Client is a goroutine-safe Kafka test client. One Client wires a
// pool of writers + readers against a fixed broker set; create one per
// test and reuse across operations.
//
// Writers are lazily created per topic and cached. Readers are created
// per Consume call (Kafka readers are stateful — group, partition,
// offset — so caching them under a single key isn't safe).
type Client struct {
	cfg     *config
	writers map[string]*kafka.Writer
	mu      atomicMap
	seq     atomic.Uint64
}

// NewClient constructs a Kafka client against the supplied brokers
// (e.g. []string{"localhost:9092"}). At least one broker is required.
//
// The returned client owns its writer pool; call Close to flush + shut
// down pending writes and drain the configured recorder.
func NewClient(brokers []string, opts ...Option) (*Client, error) {
	if len(brokers) == 0 {
		return nil, errors.New("mockarty kafka: at least one broker required")
	}
	cfg := defaultConfig()
	cfg.brokers = append(cfg.brokers, brokers...)
	for _, opt := range opts {
		opt(cfg)
	}
	return &Client{
		cfg:     cfg,
		writers: map[string]*kafka.Writer{},
	}, nil
}

// Close flushes every pooled writer and releases their resources.
// Idempotent — second call returns the cached error from the first.
func (c *Client) Close() error {
	if c == nil {
		return nil
	}
	var firstErr error
	c.mu.do(func() {
		for _, w := range c.writers {
			if err := w.Close(); err != nil && firstErr == nil {
				firstErr = err
			}
		}
		c.writers = map[string]*kafka.Writer{}
	})
	return firstErr
}

// Produce writes one message to the topic. Payload accepts the same
// shapes InvokeJSON does in protocols/grpc: []byte, string,
// json.RawMessage, or a Go value (struct/map) that will be json.Marshalled
// to bytes.
//
// Optional headers are emitted on the wire as Kafka message headers.
// Empty Key triggers Kafka's default round-robin partitioning; pass
// WithDefaultBalancer in NewClient to override.
//
// Errors are classified:
//
//   - broken: network / transport failures (cannot reach broker, etc).
//   - failed: server-side rejection (auth, topic not found, etc).
//
// Both surface in the recorded step's Status / Message.
func (c *Client) Produce(ctx context.Context, topic string, key string, payload any, headers map[string]string) error {
	if c == nil {
		return errors.New("mockarty kafka: nil client")
	}
	if topic == "" {
		return errors.New("mockarty kafka: empty topic")
	}
	// Take the start BEFORE marshalling / writer lookup so marshal-
	// error steps record a non-zero duration consistent with the
	// rest of the timeline (review B7).
	start := time.Now()
	w := c.writerFor(topic)
	body, err := marshalPayload(payload)
	if err != nil {
		c.recordStep(ctx, "produce:"+topic, start, time.Since(start), "failed", err, nil)
		return err
	}
	msg := kafka.Message{
		Key:   []byte(key),
		Value: body,
	}
	for k, v := range headers {
		msg.Headers = append(msg.Headers, kafka.Header{Key: k, Value: []byte(v)})
	}
	err = w.WriteMessages(ctx, msg)
	dur := time.Since(start)
	if err != nil {
		c.recordStep(ctx, "produce:"+topic, start, dur, classify(err), err, map[string]string{
			"key":          key,
			"payload":      capPreview(body, c.cfg.payloadCap),
			"payload_size": strconv.Itoa(len(body)),
		})
		return err
	}
	c.recordStep(ctx, "produce:"+topic, start, dur, "passed", nil, map[string]string{
		"key":          key,
		"payload":      capPreview(body, c.cfg.payloadCap),
		"payload_size": strconv.Itoa(len(body)),
	})
	return nil
}

// ConsumeOptions controls one Consume call.
type ConsumeOptions struct {
	Topic   string
	GroupID string
	// MaxMessages caps how many messages to fetch before returning.
	// Defaults to 1 (one-message-per-call mode — most CI test code
	// asserts a single message at a time).
	MaxMessages int
	// MinBytes / MaxBytes — server-side fetch shape. Defaults
	// match kafka-go defaults (1 byte / 10 MiB) when zero.
	MinBytes int
	MaxBytes int
	// StartOffset — first/last/specific. Zero = FirstOffset.
	StartOffset int64
	// Decode, if non-nil, unmarshals each consumed message's Value
	// into the supplied pointer (typically a *map[string]any or a
	// pointer to a typed struct). When nil the raw bytes are
	// available on the returned ConsumedMessage.
	Decode any
}

// ConsumedMessage carries the bytes the consumer pulled off the topic
// plus the Kafka-side coordinates (partition / offset / headers).
type ConsumedMessage struct {
	Time      time.Time
	Headers   map[string]string
	Topic     string
	Key       string
	Value     []byte
	Partition int
	Offset    int64
}

// Consume fetches up to opts.MaxMessages messages from the topic.
// Honours ctx timeout — a zero result with a deadline-exceeded ctx
// surfaces as ctx.Err(), with the step recorded as "broken".
//
// One step is recorded per Consume call (not per message) so the TCM
// timeline shows "consume:<topic>" with the total count + duration.
func (c *Client) Consume(ctx context.Context, opts ConsumeOptions) ([]ConsumedMessage, error) {
	if c == nil {
		return nil, errors.New("mockarty kafka: nil client")
	}
	if opts.Topic == "" {
		return nil, errors.New("mockarty kafka: empty topic")
	}
	if opts.MaxMessages <= 0 {
		opts.MaxMessages = 1
	}
	r := kafka.NewReader(kafka.ReaderConfig{
		Brokers:     c.cfg.brokers,
		Topic:       opts.Topic,
		GroupID:     opts.GroupID,
		MinBytes:    opts.MinBytes,
		MaxBytes:    opts.MaxBytes,
		StartOffset: opts.StartOffset,
	})
	defer r.Close()
	start := time.Now()
	out := make([]ConsumedMessage, 0, opts.MaxMessages)
	// Build the step at the end so a Decode failure can downgrade the
	// verdict from passed → failed without emitting a stale row first
	// (review B5 — TCM timelines used to show a passing consume right
	// before the decoder rejected the body).
	stepName := "consume:" + opts.Topic
	stepStatus := "passed"
	var stepErr error
	defer func() {
		c.recordStep(ctx, stepName, start, time.Since(start), stepStatus, stepErr, map[string]string{
			"count": strconv.Itoa(len(out)),
			"group": opts.GroupID,
		})
	}()
	for i := 0; i < opts.MaxMessages; i++ {
		m, err := r.ReadMessage(ctx)
		if err != nil {
			stepStatus = classify(err)
			stepErr = err
			return out, err
		}
		out = append(out, toConsumedMessage(m))
	}
	if opts.Decode != nil && len(out) > 0 {
		if err := json.Unmarshal(out[0].Value, opts.Decode); err != nil {
			stepStatus = "failed"
			stepErr = err
			return out, fmt.Errorf("mockarty kafka: decode first message into %T: %w", opts.Decode, err)
		}
	}
	return out, nil
}

// writerFor returns the cached writer for topic, creating it on first
// use. kafka-go writers are goroutine-safe so a single instance per
// topic is correct.
func (c *Client) writerFor(topic string) *kafka.Writer {
	var w *kafka.Writer
	c.mu.do(func() {
		w = c.writers[topic]
		if w != nil {
			return
		}
		w = &kafka.Writer{
			Addr:                   kafka.TCP(c.cfg.brokers...),
			Topic:                  topic,
			Balancer:               c.cfg.balancer,
			BatchTimeout:           10 * time.Millisecond,
			WriteTimeout:           c.cfg.writeTimeout,
			RequiredAcks:           c.cfg.requiredAcks,
			AllowAutoTopicCreation: c.cfg.allowAutoTopic,
		}
		c.writers[topic] = w
	})
	return w
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

func toConsumedMessage(m kafka.Message) ConsumedMessage {
	out := ConsumedMessage{
		Time:      m.Time,
		Topic:     m.Topic,
		Key:       string(m.Key),
		Value:     append([]byte(nil), m.Value...),
		Partition: m.Partition,
		Offset:    m.Offset,
	}
	if len(m.Headers) > 0 {
		out.Headers = make(map[string]string, len(m.Headers))
		for _, h := range m.Headers {
			out.Headers[h.Key] = string(h.Value)
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
		return nil, fmt.Errorf("mockarty kafka: marshal payload: %w", err)
	}
	return b, nil
}

// classify maps a kafka-go error to a telemetry step status.
//
// Decision tree (review B2 fix — the previous default was "failed",
// which masked real broker-unreachable / TLS / DNS errors as if they
// were test assertion failures):
//
//   - ctx.Canceled / DeadlineExceeded               → "broken" (env)
//   - typed kafka.Error (protocol-level rejection)  → "failed"
//   - net.Error / io.EOF / everything else          → "broken" (env)
//
// "Broken" is the conservative default — when the network or broker
// is unreachable, the run is not really telling us anything about the
// system under test, and flaky CI doesn't deserve to be coloured red
// like a genuine assertion failure.
func classify(err error) string {
	if err == nil {
		return "passed"
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return "broken"
	}
	var kerr kafka.Error
	if errors.As(err, &kerr) {
		return "failed"
	}
	return "broken"
}

// capPreview is a thin alias to telemetry.CapPreview kept so the call
// sites in this package don't have to spell out the full import path.
// All UTF-8 boundary + truncation marker concerns live in the shared
// helper — change there, propagates everywhere.
func capPreview(body []byte, cap int) string {
	return telemetry.CapPreview(body, cap)
}
