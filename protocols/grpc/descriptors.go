// Copyright (c) 2026 Mockarty. All rights reserved.
// Licensed under the Mockarty SDK License Agreement. See LICENSE file for details.

package grpc

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/jhump/protoreflect/desc"
	"github.com/jhump/protoreflect/desc/protoparse"
	"github.com/jhump/protoreflect/grpcreflect"
	"google.golang.org/grpc"
	reflectpb "google.golang.org/grpc/reflection/grpc_reflection_v1alpha"
)

// descriptorSource is the narrow interface the client uses to resolve
// methods + list the service surface. Two production implementations:
//
//   - fileSource: parses .proto files (paths from WithProtoFile).
//   - reflectSource: queries the server's gRPC reflection service.
//
// combinedSource tries fileSource first (cheaper, deterministic), falls
// back to reflectSource. The fallback order is intentional: when the
// user pinned a .proto file they likely want THAT exact schema, not
// whatever the server happens to expose today.
type descriptorSource interface {
	FindMethod(ctx context.Context, fullMethod string) (*desc.MethodDescriptor, error)
	ListServices(ctx context.Context) ([]string, error)
	Close() error
}

// errMethodNotFound is the sentinel returned when no source can resolve
// the requested method. Callers catch it via errors.Is to fall back to
// the next source.
var errMethodNotFound = errors.New("grpc descriptor: method not found")

// ---------- fileSource ----------

// fileSource holds descriptors parsed from .proto files. Parsing is
// done once at construction; the resulting descriptors are read-only
// and safe for concurrent lookups.
type fileSource struct {
	files []*desc.FileDescriptor
}

func newFileSource(paths, importDirs []string) (*fileSource, error) {
	if len(paths) == 0 {
		return nil, nil
	}
	p := protoparse.Parser{
		ImportPaths:           importDirs,
		IncludeSourceCodeInfo: false,
	}
	files, err := p.ParseFiles(paths...)
	if err != nil {
		return nil, fmt.Errorf("grpc descriptor: parse proto files: %w", err)
	}
	return &fileSource{files: files}, nil
}

func (s *fileSource) FindMethod(_ context.Context, fullMethod string) (*desc.MethodDescriptor, error) {
	if s == nil {
		return nil, errMethodNotFound
	}
	service, method, err := splitMethod(fullMethod)
	if err != nil {
		return nil, err
	}
	for _, f := range s.files {
		if sd := f.FindService(service); sd != nil {
			if md := sd.FindMethodByName(method); md != nil {
				return md, nil
			}
		}
	}
	return nil, fmt.Errorf("%w: %s", errMethodNotFound, fullMethod)
}

func (s *fileSource) ListServices(_ context.Context) ([]string, error) {
	if s == nil {
		return nil, nil
	}
	var out []string
	for _, f := range s.files {
		for _, sd := range f.GetServices() {
			out = append(out, sd.GetFullyQualifiedName())
		}
	}
	return out, nil
}

func (s *fileSource) Close() error { return nil }

// ---------- reflectSource ----------

// reflectSource talks gRPC server reflection. Single connection per
// client; the underlying grpcreflect.Client is created on first use
// and cached so List/Find don't open a fresh stream every time.
type reflectSource struct {
	cc    *grpc.ClientConn
	cli   *grpcreflect.Client
	cliMu sync.Mutex
}

func newReflectSource(cc *grpc.ClientConn) *reflectSource {
	return &reflectSource{cc: cc}
}

func (s *reflectSource) ensureClient(ctx context.Context) *grpcreflect.Client {
	s.cliMu.Lock()
	defer s.cliMu.Unlock()
	if s.cli == nil {
		// V1Alpha covers the vast majority of servers including the
		// stable grpc-go reflection backend. The newer V1 protocol is
		// wire-compatible for what we need, so jhump's library
		// transparently falls back when the server speaks the legacy
		// protocol.
		s.cli = grpcreflect.NewClient(ctx, reflectpb.NewServerReflectionClient(s.cc))
	}
	return s.cli
}

func (s *reflectSource) FindMethod(ctx context.Context, fullMethod string) (*desc.MethodDescriptor, error) {
	if s == nil {
		return nil, errMethodNotFound
	}
	service, method, err := splitMethod(fullMethod)
	if err != nil {
		return nil, err
	}
	cli := s.ensureClient(ctx)
	sd, err := cli.ResolveService(service)
	if err != nil {
		return nil, fmt.Errorf("%w: reflection ResolveService %q: %v", errMethodNotFound, service, err)
	}
	md := sd.FindMethodByName(method)
	if md == nil {
		return nil, fmt.Errorf("%w: %s", errMethodNotFound, fullMethod)
	}
	return md, nil
}

func (s *reflectSource) ListServices(ctx context.Context) ([]string, error) {
	if s == nil {
		return nil, nil
	}
	cli := s.ensureClient(ctx)
	return cli.ListServices()
}

func (s *reflectSource) Close() error {
	s.cliMu.Lock()
	defer s.cliMu.Unlock()
	if s.cli != nil {
		s.cli.Reset()
		s.cli = nil
	}
	return nil
}

// ---------- combinedSource ----------

// combinedSource probes file source first, reflection second. Either
// can be nil — the surviving one carries all the lookups.
type combinedSource struct {
	file *fileSource
	refl *reflectSource
}

func (s *combinedSource) FindMethod(ctx context.Context, fullMethod string) (*desc.MethodDescriptor, error) {
	if s.file != nil {
		md, err := s.file.FindMethod(ctx, fullMethod)
		if err == nil {
			return md, nil
		}
		if !errors.Is(err, errMethodNotFound) {
			return nil, err
		}
	}
	if s.refl != nil {
		return s.refl.FindMethod(ctx, fullMethod)
	}
	return nil, fmt.Errorf("%w: no descriptor source has this method (registered file paths: %d, reflection enabled: %v)",
		errMethodNotFound, fileCount(s.file), s.refl != nil)
}

func (s *combinedSource) ListServices(ctx context.Context) ([]string, error) {
	seen := map[string]struct{}{}
	var out []string
	if s.file != nil {
		names, _ := s.file.ListServices(ctx)
		for _, n := range names {
			if _, dup := seen[n]; dup {
				continue
			}
			seen[n] = struct{}{}
			out = append(out, n)
		}
	}
	if s.refl != nil {
		names, err := s.refl.ListServices(ctx)
		if err != nil && len(out) == 0 {
			return nil, err
		}
		for _, n := range names {
			if _, dup := seen[n]; dup {
				continue
			}
			seen[n] = struct{}{}
			out = append(out, n)
		}
	}
	return out, nil
}

func (s *combinedSource) Close() error {
	var firstErr error
	if s.file != nil {
		if err := s.file.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	if s.refl != nil {
		if err := s.refl.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

// ---------- helpers ----------

// splitMethod normalises a user-facing method ref into
// (fullyQualifiedService, methodName). Accepts both:
//
//	acme.UserService/GetUser
//	/acme.UserService/GetUser  (gRPC-style with leading slash)
//
// Returns an error on empty / malformed input. Method names with dots
// (rare but legal in some legacy services) are preserved verbatim.
func splitMethod(s string) (string, string, error) {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "/")
	if s == "" {
		return "", "", errors.New("grpc descriptor: empty method")
	}
	// Use last '/' so service names containing '/' (which is invalid
	// per gRPC spec anyway, but be defensive) don't break the split.
	idx := strings.LastIndex(s, "/")
	if idx <= 0 || idx == len(s)-1 {
		return "", "", fmt.Errorf("grpc descriptor: malformed method %q (expected \"package.Service/Method\")", s)
	}
	return s[:idx], s[idx+1:], nil
}

func fileCount(f *fileSource) int {
	if f == nil {
		return 0
	}
	return len(f.files)
}
