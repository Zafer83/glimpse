package crawler

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestExpandTilde(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("cannot get home dir: %v", err)
	}

	tests := []struct {
		name string
		in   string
		want string
	}{
		{"bare tilde", "~", home},
		{"tilde slash", "~/projects/foo", filepath.Join(home, "projects/foo")},
		{"absolute path unchanged", "/usr/local/bin", "/usr/local/bin"},
		{"relative path unchanged", "../foo/bar", "../foo/bar"},
		{"tilde in middle unchanged", "/foo/~/bar", "/foo/~/bar"},
		{"empty string", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := expandTilde(tt.in)
			if got != tt.want {
				t.Errorf("expandTilde(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestCollectCode_BasicFiles(t *testing.T) {
	dir := t.TempDir()

	// Create a .go file, a .md doc, a .txt doc, and a .log file (should be ignored).
	os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\nfunc main() {}"), 0644)
	os.WriteFile(filepath.Join(dir, "README.md"), []byte("# Hello"), 0644)
	os.WriteFile(filepath.Join(dir, "notes.txt"), []byte("project notes"), 0644)
	os.WriteFile(filepath.Join(dir, "debug.log"), []byte("ignored"), 0644)

	result, err := CollectCode(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(result, "main.go") {
		t.Error("expected main.go in output")
	}
	if !strings.Contains(result, "README.md") {
		t.Error("expected README.md in output")
	}
	if !strings.Contains(result, "notes.txt") {
		t.Error("expected notes.txt in output (.txt is now a doc format)")
	}
	if strings.Contains(result, "debug.log") {
		t.Error("debug.log should not be in output (unsupported extension)")
	}
	if !strings.Contains(result, "DOCUMENTATION FIRST") {
		t.Error("expected documentation-first header")
	}
}

func TestCollectCode_SkipDirs(t *testing.T) {
	dir := t.TempDir()

	// Create files in directories that should be skipped.
	for _, skip := range []string{"node_modules", ".git", "dist", "build", "vendor"} {
		p := filepath.Join(dir, skip)
		os.MkdirAll(p, 0755)
		os.WriteFile(filepath.Join(p, "index.js"), []byte("skip me"), 0644)
	}
	// And one valid file at root.
	os.WriteFile(filepath.Join(dir, "app.py"), []byte("print('hello')"), 0644)

	result, err := CollectCode(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(result, "app.py") {
		t.Error("expected app.py in output")
	}
	if strings.Contains(result, "index.js") {
		t.Error("files inside skipped directories should not appear")
	}
}

func TestCollectCode_EmptyDir(t *testing.T) {
	dir := t.TempDir()

	result, err := CollectCode(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Only the header, no file content.
	if strings.Contains(result, "CODE FILE") || strings.Contains(result, "DOC FILE") {
		t.Error("empty dir should produce no file entries")
	}
}

func TestCollectCode_NonexistentPath(t *testing.T) {
	_, err := CollectCode("/nonexistent/path/that/does/not/exist")
	if err == nil {
		t.Error("expected error for nonexistent path")
	}
}

func TestCollectCode_SkipLockFiles(t *testing.T) {
	dir := t.TempDir()

	os.WriteFile(filepath.Join(dir, "package-lock.json"), []byte(`{"lockfileVersion":3}`), 0644)
	os.WriteFile(filepath.Join(dir, "yarn.lock"), []byte("# yarn"), 0644)
	os.WriteFile(filepath.Join(dir, "go.sum"), []byte("h1:abc"), 0644)
	os.WriteFile(filepath.Join(dir, "package.json"), []byte(`{"name":"ok"}`), 0644)

	result, err := CollectCode(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if strings.Contains(result, "package-lock.json") {
		t.Error("package-lock.json should be skipped")
	}
	if strings.Contains(result, "yarn.lock") {
		t.Error("yarn.lock should be skipped")
	}
	if strings.Contains(result, "go.sum") {
		t.Error("go.sum should be skipped")
	}
	if !strings.Contains(result, "package.json") {
		t.Error("package.json should be included")
	}
}

func TestCollectProject_TieredOutput(t *testing.T) {
	dir := t.TempDir()

	// Docs
	os.WriteFile(filepath.Join(dir, "README.md"), []byte("# Project\nOverview here"), 0644)
	os.WriteFile(filepath.Join(dir, "notes.txt"), []byte("Design notes"), 0644)

	// Business logic
	os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\nfunc main() {}"), 0644)
	os.WriteFile(filepath.Join(dir, "service.py"), []byte("class Service:\n    pass"), 0644)

	// Support: test file
	os.WriteFile(filepath.Join(dir, "main_test.go"), []byte("package main\nfunc TestMain(t *testing.T) {}"), 0644)

	// Support: config
	os.WriteFile(filepath.Join(dir, "config.yaml"), []byte("key: value"), 0644)

	content, err := CollectProject(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	docs, biz, sup := content.Stats()
	if docs != 2 {
		t.Errorf("expected 2 docs, got %d", docs)
	}
	if biz != 2 {
		t.Errorf("expected 2 business files, got %d", biz)
	}
	if sup != 2 {
		t.Errorf("expected 2 support files, got %d", sup)
	}

	// README should sort before notes.txt.
	if len(content.Docs) >= 2 {
		if !strings.Contains(content.Docs[0].Path, "README") {
			t.Errorf("README should be first doc, got: %s", content.Docs[0].Path)
		}
	}
}

func TestCollectProject_DocxSupport(t *testing.T) {
	dir := t.TempDir()

	// Create a minimal .rst doc.
	os.WriteFile(filepath.Join(dir, "guide.rst"), []byte("Title\n=====\nSome reStructuredText."), 0644)

	content, err := CollectProject(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	docs, _, _ := content.Stats()
	if docs != 1 {
		t.Errorf("expected 1 doc (.rst), got %d", docs)
	}
}
