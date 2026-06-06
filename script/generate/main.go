package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func main() {
	srcDir := filepath.Join("sdk")
	dstDir := filepath.Join("internal", "runner", "sdk")

	if err := syncDir(srcDir, dstDir); err != nil {
		fmt.Fprintln(os.Stderr, "sync failed:", err)
		os.Exit(1)
	}
	fmt.Println("sync complete:", srcDir, "->", dstDir)
}

func syncDir(srcDir, dstDir string) error {
	absSrc, err := filepath.Abs(srcDir)
	if err != nil {
		return fmt.Errorf("abs src: %w", err)
	}
	absDst, err := filepath.Abs(dstDir)
	if err != nil {
		return fmt.Errorf("abs dst: %w", err)
	}

	entries, err := os.ReadDir(absSrc)
	if err != nil {
		return fmt.Errorf("read src dir: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasSuffix(name, ".go") {
			continue
		}
		if strings.HasSuffix(name, "_test.go") {
			continue
		}

		srcPath := filepath.Join(absSrc, name)
		dstPath := filepath.Join(absDst, name)

		data, err := os.ReadFile(srcPath)
		if err != nil {
			return fmt.Errorf("read %s: %w", name, err)
		}

		if err := os.WriteFile(dstPath, data, 0644); err != nil {
			return fmt.Errorf("write %s: %w", name, err)
		}

		fmt.Println("  copied:", name)
	}

	return nil
}
