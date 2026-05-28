package lint_test

import (
	"os"
	"path/filepath"
	"testing"
)

// tmpWorkspace builds a tiny on-disk workspace in t.TempDir() and returns
// its root. Nested directories implied by the keys of files are created
// automatically; values are written verbatim as the file body. Mirrors the
// fixtureWorkspace helper in the root sirena package's tests; duplicated
// here because Go does not export test helpers across packages.
func tmpWorkspace(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	for name, content := range files {
		full := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}
