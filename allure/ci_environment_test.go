package allure

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCIEnvironmentOmitsPrivateDelivery(t *testing.T) {
	dir := t.TempDir()
	env := map[string]string{"MOCKARTY_CI_CONTEXT": "private-envelope", "alias": "prefix-private-envelope", "safe": "kept"}
	w := NewFileWriter(dir)
	if err := w.WriteEnvironment(env); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(dir, "environment.properties"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(b), "MOCKARTY_CI_CONTEXT") || strings.Contains(string(b), "private-envelope") || !strings.Contains(string(b), "safe=kept") {
		t.Fatal("capability persisted into framework environment")
	}
	if env["alias"] != "prefix-private-envelope" {
		t.Fatal("caller mutated")
	}
}
