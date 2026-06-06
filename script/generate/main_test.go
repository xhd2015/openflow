package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSyncDir(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "sync-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	src := filepath.Join(tmpDir, "src")
	dst := filepath.Join(tmpDir, "dst")
	if err := os.MkdirAll(src, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(dst, 0755); err != nil {
		t.Fatal(err)
	}

	mustWrite := func(dir, name, content string) {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}

	mustWrite(src, "agent.go", "package sdk\n// agent")
	mustWrite(src, "shell.go", "package sdk\n// shell")
	mustWrite(src, "sdk_test.go", "package sdk\n// test")
	mustWrite(src, "go.mod", "module x")
	if err := os.Mkdir(filepath.Join(src, "vendor"), 0755); err != nil {
		t.Fatal(err)
	}

	if err := syncDir(src, dst); err != nil {
		t.Fatal(err)
	}

	assertCopied := func(name, expectedContent string) {
		path := filepath.Join(dst, name)
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("expected %s to exist: %v", name, err)
		}
		if string(data) != expectedContent {
			t.Fatalf("expected %s content %q, got %q", name, expectedContent, string(data))
		}
	}

	assertNotExist := func(name string) {
		path := filepath.Join(dst, name)
		if _, err := os.Stat(path); err == nil {
			t.Fatalf("expected %s to not exist", name)
		}
	}

	assertCopied("agent.go", "package sdk\n// agent")
	assertCopied("shell.go", "package sdk\n// shell")
	assertNotExist("sdk_test.go")
	assertNotExist("go.mod")
	assertNotExist("vendor")
}

func TestSyncDirSkipTestFiles(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "sync-skip-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	src := filepath.Join(tmpDir, "src")
	dst := filepath.Join(tmpDir, "dst")
	os.MkdirAll(src, 0755)
	os.MkdirAll(dst, 0755)

	names := []string{
		"a.go", "b_test.go", "c_test.go", "d.go",
	}
	for _, name := range names {
		os.WriteFile(filepath.Join(src, name), []byte(name), 0644)
	}

	if err := syncDir(src, dst); err != nil {
		t.Fatal(err)
	}

	entries, err := os.ReadDir(dst)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 synced files, got %d", len(entries))
	}
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), "_test.go") {
			t.Fatalf("unexpected test file in dst: %s", e.Name())
		}
	}
}

func TestMain(m *testing.M) {
	os.Exit(m.Run())
}
