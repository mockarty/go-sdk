// Copyright (c) 2026 Mockarty. All rights reserved.
// Licensed under the MIT License. See LICENSE file for details.

package grpc

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/jhump/protoreflect/desc"
	"github.com/jhump/protoreflect/dynamic"
	"github.com/jhump/protoreflect/dynamic/grpcdynamic"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// Client is a reflection-driven gRPC test client. It is goroutine-safe;
// the typical test wires ONE Client per service and reuses it across
// every RPC the test fires.
type Client struct {
	cc     *grpc.ClientConn
	cfg    *config
	src    descriptorSource
	target string
	stub   grpcdynamic.Stub
	seq    atomic.Uint64
}

// Dial connects to target (host:port) and returns a Client ready for
// InvokeJSON. The ctx bounds the dial AND the descriptor source
// initialisation (reflection ListServices probe when WithReflection
// is on).
//
// The connection is plaintext by default — pass WithTLS for prod
// endpoints. Pass WithDialOption for keepalive / balancer tuning.
//
// Close MUST be called when the Client is no longer needed; it
// releases the gRPC connection AND drains a recorder if one is wired.
func Dial(ctx context.Context, target string, opts ...Option) (*Client, error) {
	if target == "" {
		return nil, errors.New("mockartygrpc: empty dial target")
	}
	cfg := defaultConfig()
	for _, opt := range opts {
		opt(cfg)
	}
	dialOpts := append([]grpc.DialOption{}, cfg.dialOpts...)
	if cfg.creds != nil {
		dialOpts = append(dialOpts, grpc.WithTransportCredentials(cfg.creds))
	} else {
		dialOpts = append(dialOpts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	}
	cc, err := grpc.DialContext(ctx, target, dialOpts...)
	if err != nil {
		return nil, fmt.Errorf("mockartygrpc: dial %s: %w", target, err)
	}

	// Build descriptor source: file (if any) + reflection (when enabled).
	src := &combinedSource{}
	if len(cfg.protoFiles) > 0 {
		fs, ferr := newFileSource(cfg.protoFiles, cfg.importDirs)
		if ferr != nil {
			_ = cc.Close()
			return nil, ferr
		}
		src.file = fs
	}
	if cfg.useReflection {
		src.refl = newReflectSource(cc)
	}

	return &Client{
		cc:     cc,
		cfg:    cfg,
		src:    src,
		target: target,
		stub:   grpcdynamic.NewStub(cc),
	}, nil
}

// Conn returns the underlying *grpc.ClientConn. Useful when a test
// needs to mix in a generated stub for a method the dynamic path
// can't express (e.g. interceptor instrumentation). Don't Close the
// returned connection — Client.Close owns its lifecycle.
func (c *Client) Conn() *grpc.ClientConn { return c.cc }

// Target returns the dial target the client was constructed with.
func (c *Client) Target() string { return c.target }

// Close releases the gRPC connection. Idempotent; second call returns
// the cached error from the first.
func (c *Client) Close() error {
	if c == nil {
		return nil
	}
	if c.src != nil {
		_ = c.src.Close()
	}
	if c.cc != nil {
		return c.cc.Close()
	}
	return nil
}

// InvokeJSON calls a unary RPC by full method name (e.g.
// "acme.UserService/GetUser"). The request is JSON-shaped: it can be a
// []byte raw JSON, a string, a map[string]any, or any struct with
// json tags. The response is decoded back into resp via the same
// json package — resp must be a pointer.
//
// Errors include:
//
//   - method-not-found (no descriptor source could resolve the method);
//   - JSON↔protobuf shape errors (caller-facing — the user's payload
//     didn't match the schema);
//   - transport / status errors from gRPC.
//
// Every call is timed and reported to the configured StepRecorder.
// On gRPC status errors the step is recorded as "failed" with the
// status message; transport errors are recorded as "broken".
func (c *Client) InvokeJSON(ctx context.Context, fullMethod string, req, resp any) error {
	if c == nil || c.cc == nil {
		return errors.New("mockartygrpc: nil client")
	}
	// Per-call timeout fallback when ctx has no deadline.
	if _, hasDeadline := ctx.Deadline(); !hasDeadline && c.cfg.timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, c.cfg.timeout)
		defer cancel()
	}
	// Inject default metadata.
	if len(c.cfg.metadata) > 0 {
		pairs := make([]string, 0, len(c.cfg.metadata)*2)
		for k, v := range c.cfg.metadata {
			pairs = append(pairs, k, v)
		}
		ctx = metadata.AppendToOutgoingContext(ctx, pairs...)
	}

	seq := c.seq.Add(1)
	started := time.Now()
	step := Step{
		StartedAt: started,
		Key:       stepKey(fullMethod, seq),
		Name:      fullMethod,
	}
	finishStep := func(err error) {
		step.EndedAt = time.Now()
		step.Duration = step.EndedAt.Sub(step.StartedAt)
		switch {
		case err == nil:
			step.Status = "passed"
		case status.Code(err) != 0 && status.Code(err).String() != "Unknown":
			step.Status = "failed"
			step.Message = err.Error()
		default:
			step.Status = "broken"
			step.Message = err.Error()
		}
		c.cfg.recorder.Record(ctx, step)
	}

	md, err := c.src.FindMethod(ctx, fullMethod)
	if err != nil {
		finishStep(err)
		return err
	}
	if md.IsClientStreaming() || md.IsServerStreaming() {
		err = fmt.Errorf("mockartygrpc: %s is a streaming method — use NewServerStream for server-streaming (client/bidi streaming not supported in v1)", fullMethod)
		finishStep(err)
		return err
	}

	reqMsg := dynamic.NewMessage(md.GetInputType())
	if err = unmarshalJSONInto(req, reqMsg); err != nil {
		finishStep(err)
		return err
	}
	c.captureParam(&step, "request", req)

	respMsg, err := c.stub.InvokeRpc(ctx, md, reqMsg)
	if err != nil {
		finishStep(err)
		return err
	}
	respJSON, jerr := respMsg.(*dynamic.Message).MarshalJSON()
	if jerr != nil {
		finishStep(jerr)
		return fmt.Errorf("mockartygrpc: marshal response: %w", jerr)
	}
	c.captureParam(&step, "response", json.RawMessage(respJSON))
	finishStep(nil)
	if resp == nil {
		return nil
	}
	if err = json.Unmarshal(respJSON, resp); err != nil {
		return fmt.Errorf("mockartygrpc: decode response into %T: %w", resp, err)
	}
	return nil
}

// ListServices returns the fully-qualified names of every gRPC service
// the descriptor source knows about. Useful for sanity-checks in test
// setup: "is the mock actually wired to acme.UserService?".
func (c *Client) ListServices(ctx context.Context) ([]string, error) {
	if c == nil || c.src == nil {
		return nil, errors.New("mockartygrpc: nil client")
	}
	return c.src.ListServices(ctx)
}

// ListMethods returns the method names of a service. Empty result
// without an error means the service exists but has no methods (rare).
func (c *Client) ListMethods(ctx context.Context, service string) ([]string, error) {
	if c == nil || c.src == nil {
		return nil, errors.New("mockartygrpc: nil client")
	}
	// Descend through the combined source's underlyings — both
	// fileSource and reflectSource expose a *desc.ServiceDescriptor
	// whose GetMethods() returns []*desc.MethodDescriptor, so one
	// helper covers both branches.
	collect := func(sd *desc.ServiceDescriptor) []string {
		mds := sd.GetMethods()
		out := make([]string, 0, len(mds))
		for _, m := range mds {
			out = append(out, m.GetName())
		}
		return out
	}
	if combined, ok := c.src.(*combinedSource); ok {
		if combined.file != nil {
			for _, f := range combined.file.files {
				if sd := f.FindService(service); sd != nil {
					return collect(sd), nil
				}
			}
		}
		if combined.refl != nil {
			cli := combined.refl.ensureClient(ctx)
			sd, err := cli.ResolveService(service)
			if err != nil {
				return nil, fmt.Errorf("mockartygrpc: resolve service %q: %w", service, err)
			}
			return collect(sd), nil
		}
	}
	return nil, fmt.Errorf("mockartygrpc: cannot list methods of %q (no descriptor source)", service)
}

// captureParam adds a truncated JSON snapshot of the value to the
// step's Parameters under the supplied key. Honors payloadCap from
// the config — set to 0 via WithPayloadCaptureBytes(0) to opt out.
func (c *Client) captureParam(step *Step, key string, value any) {
	if c.cfg.payloadCap == 0 {
		return
	}
	if step.Parameters == nil {
		step.Parameters = make(map[string]string, 2)
	}
	var data []byte
	switch v := value.(type) {
	case []byte:
		data = v
	case string:
		data = []byte(v)
	case json.RawMessage:
		data = v
	default:
		b, err := json.Marshal(v)
		if err != nil {
			step.Parameters[key] = "<marshal error: " + err.Error() + ">"
			return
		}
		data = b
	}
	if len(data) > c.cfg.payloadCap {
		step.Parameters[key] = string(data[:c.cfg.payloadCap]) + "…(truncated " + strconv.Itoa(len(data)-c.cfg.payloadCap) + "B)"
		return
	}
	step.Parameters[key] = string(data)
}

// unmarshalJSONInto decodes value into the dynamic.Message via JSON.
// Accepts []byte, string, json.RawMessage, map / struct (json-marshalled).
// Nil value yields an empty (zero) message — legal for methods whose
// input is google.protobuf.Empty or has no required fields.
func unmarshalJSONInto(value any, m *dynamic.Message) error {
	if value == nil {
		return nil
	}
	var data []byte
	switch v := value.(type) {
	case nil:
		return nil
	case []byte:
		data = v
	case string:
		data = []byte(v)
	case json.RawMessage:
		data = v
	default:
		b, err := json.Marshal(v)
		if err != nil {
			return fmt.Errorf("mockartygrpc: marshal request as JSON: %w", err)
		}
		data = b
	}
	if len(data) == 0 {
		return nil
	}
	if err := m.UnmarshalJSON(data); err != nil {
		return fmt.Errorf("mockartygrpc: bind JSON to proto %s: %w", m.GetMessageDescriptor().GetFullyQualifiedName(), err)
	}
	return nil
}

