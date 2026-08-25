package main

import (
	"os"
	"path/filepath"
)

// resolveDBPath anchors the untouched default at the repo root so the API can
// be started from any working directory. Explicit DB_PATH is used verbatim.
func resolveDBPath(p string) string {
	if p != "./data/personal-os.db" {
		return p
	}
	dir, err := filepath.Abs(".")
	if err != nil {
		return p
	}
	for i := 0; i < 6; i++ {
		if _, statErr := os.Stat(filepath.Join(dir, ".git")); statErr == nil {
			return filepath.Join(dir, "data", "personal-os.db")
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return p
}
