package models

import (
	"os"
	"path/filepath"
	"testing"
)

func TestBackupRoots(t *testing.T) {
	dir := t.TempDir()
	nested := filepath.Join(dir, "nested")
	if err := os.Mkdir(nested, 0o700); err != nil {
		t.Fatal(err)
	}
	roots, err := BackupRoots([]string{nested, dir, dir})
	if err != nil {
		t.Fatal(err)
	}
	if len(roots) != 1 || roots[0] != dir {
		t.Fatalf("roots=%v", roots)
	}
	file := filepath.Join(dir, "file")
	if err := os.WriteFile(file, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	for _, paths := range [][]string{nil, {""}, {file}, {filepath.Join(dir, "absent")}} {
		if _, err := BackupRoots(paths); err == nil {
			t.Fatalf("accepted %v", paths)
		}
	}
}
