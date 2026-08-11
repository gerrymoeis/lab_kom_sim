package versioner

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNew_ReturnsNilOnMissingDir(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "does-not-exist")
	v, err := New(missing)
	if err == nil {
		t.Fatal("expected error for missing static dir")
	}
	if v != nil {
		t.Fatalf("expected nil Versioner on error, got %+v", v)
	}
}

func TestNew_BuildsHashesWhenDirExists(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.css"), []byte("body{}"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "b.js"), []byte("console.log(1);"), 0644); err != nil {
		t.Fatal(err)
	}
	v, err := New(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if v == nil {
		t.Fatal("expected non-nil Versioner")
	}
	if _, ok := v.hashes["a.css"]; !ok {
		t.Fatal("expected hash for a.css")
	}
	if _, ok := v.hashes["b.js"]; !ok {
		t.Fatal("expected hash for b.js")
	}
	if url := v.URL("a.css"); url == "/static/a.css" {
		t.Fatalf("expected hashed URL for a.css, got %q", url)
	}
	if url := v.URL("unknown.css"); url != "/static/unknown.css" {
		t.Fatalf("expected plain URL for unhashed path, got %q", url)
	}
}
