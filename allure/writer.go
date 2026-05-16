// Copyright (c) 2026 Mockarty. All rights reserved.
// Licensed under the Mockarty Software License Agreement.
// See LICENSE file in the project root for full license text.

package allure

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// ResultsDirEnv is the environment variable consulted for the default
// allure-results directory. Matches the Python (`ALLURE_RESULTS_DIR`) and
// Java (`-Dallure.results.directory`) SDK conventions.
const ResultsDirEnv = "ALLURE_RESULTS_DIR"

// DefaultResultsDir is the directory used when neither the environment
// variable nor [WithResultsDir] specifies one.
const DefaultResultsDir = "allure-results"

// ResolveResultsDir picks the effective output directory, honouring (in
// order): the explicit dir argument, [ResultsDirEnv], [DefaultResultsDir].
func ResolveResultsDir(explicit string) string {
	if explicit != "" {
		return explicit
	}
	if v := os.Getenv(ResultsDirEnv); v != "" {
		return v
	}
	return DefaultResultsDir
}

// ResultWriter is the persistence interface used by the scope. The default
// implementation writes JSON + attachment blobs to a filesystem directory;
// tests can swap in an in-memory writer for assertion convenience.
type ResultWriter interface {
	WriteResult(r Result) error
	RegisterAttachment(filename string, content []byte)
}

// FileWriter writes Allure result JSON files and accompanying attachments
// to a target directory. Safe for concurrent use.
type FileWriter struct {
	attachments map[string][]byte
	dir         string
	mu          sync.Mutex
}

// NewFileWriter creates a writer rooted at dir. The directory is created
// lazily on the first WriteResult call so unused writers don't litter
// empty folders.
func NewFileWriter(dir string) *FileWriter {
	return &FileWriter{
		dir:         dir,
		attachments: make(map[string][]byte),
	}
}

// Dir returns the configured output directory.
func (w *FileWriter) Dir() string { return w.dir }

// RegisterAttachment stashes the bytes for a later WriteResult call.
func (w *FileWriter) RegisterAttachment(filename string, content []byte) {
	w.mu.Lock()
	defer w.mu.Unlock()
	// Copy the slice — caller may mutate the original buffer.
	buf := make([]byte, len(content))
	copy(buf, content)
	w.attachments[filename] = buf
}

// WriteResult serialises the result and any pending attachments. The
// per-result attachments map is flushed and reset on success so each test's
// attachments live independently.
func (w *FileWriter) WriteResult(r Result) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if err := os.MkdirAll(w.dir, 0o755); err != nil {
		return fmt.Errorf("allure: create results dir: %w", err)
	}

	resultPath := filepath.Join(w.dir, r.UUID+"-result.json")
	data, err := json.Marshal(r)
	if err != nil {
		return fmt.Errorf("allure: marshal result %s: %w", r.UUID, err)
	}
	if err := os.WriteFile(resultPath, data, 0o644); err != nil {
		return fmt.Errorf("allure: write %s: %w", resultPath, err)
	}

	// Collect referenced attachment filenames so we only write the ones the
	// result actually points at. Anything else is leaked metadata — better
	// dropped than written orphaned to disk.
	referenced := make(map[string]struct{})
	collect := func(atts []AllureAttachment) {
		for _, a := range atts {
			if a.Source != "" {
				referenced[a.Source] = struct{}{}
			}
		}
	}
	collect(r.Attachments)
	var walk func(steps []AllureStep)
	walk = func(steps []AllureStep) {
		for i := range steps {
			collect(steps[i].Attachments)
			walk(steps[i].Steps)
		}
	}
	walk(r.Steps)

	for fname, content := range w.attachments {
		if _, ok := referenced[fname]; !ok {
			continue
		}
		p := filepath.Join(w.dir, fname)
		if err := os.WriteFile(p, content, 0o644); err != nil {
			return fmt.Errorf("allure: write attachment %s: %w", p, err)
		}
		delete(w.attachments, fname)
	}

	return nil
}

// WriteExecutor writes executor.json (once per CI run).
func (w *FileWriter) WriteExecutor(e Executor) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if err := os.MkdirAll(w.dir, 0o755); err != nil {
		return err
	}
	return writeJSONFile(filepath.Join(w.dir, "executor.json"), e)
}

// WriteCategories writes categories.json (once per CI run).
func (w *FileWriter) WriteCategories(cats []Category) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if err := os.MkdirAll(w.dir, 0o755); err != nil {
		return err
	}
	return writeJSONFile(filepath.Join(w.dir, "categories.json"), cats)
}

// WriteContainer writes a `<uuid>-container.json` file (rare — only when a
// shared fixture wraps multiple tests).
func (w *FileWriter) WriteContainer(c Container) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if err := os.MkdirAll(w.dir, 0o755); err != nil {
		return err
	}
	if c.UUID == "" {
		return fmt.Errorf("allure: container UUID is required")
	}
	return writeJSONFile(filepath.Join(w.dir, c.UUID+"-container.json"), c)
}

func writeJSONFile(path string, v any) error {
	data, err := json.Marshal(v)
	if err != nil {
		return fmt.Errorf("allure: marshal %s: %w", filepath.Base(path), err)
	}
	return os.WriteFile(path, data, 0o644)
}

// extForMime is a tiny MIME → extension table covering the formats Allure's
// UI knows how to render natively. Unknown types fall back to ".dat" so the
// CLI still finds the file.
func extForMime(mime string) string {
	switch strings.ToLower(strings.TrimSpace(mime)) {
	case "application/json", "application/problem+json":
		return ".json"
	case "application/xml", "text/xml":
		return ".xml"
	case "text/plain", "":
		return ".txt"
	case "text/html":
		return ".html"
	case "text/csv":
		return ".csv"
	case "text/yaml", "application/yaml", "application/x-yaml":
		return ".yaml"
	case "image/png":
		return ".png"
	case "image/jpeg", "image/jpg":
		return ".jpg"
	case "image/gif":
		return ".gif"
	case "image/svg+xml":
		return ".svg"
	case "application/pdf":
		return ".pdf"
	case "application/octet-stream":
		return ".bin"
	}
	return ".dat"
}
