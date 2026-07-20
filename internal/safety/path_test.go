package safety

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
)

func TestValidateArtifactIdentifier(t *testing.T) {
	t.Parallel()
	for _, value := range []string{"unit", "run-001", "rule_v2", "A1"} {
		if err := ValidateArtifactIdentifier("test id", value); err != nil {
			t.Errorf("expected %q to be valid: %v", value, err)
		}
	}
	for _, value := range []string{"", ".", "..", "../unit", "/tmp/unit", "nested/unit", `nested\unit`, "with space", "규칙"} {
		if err := ValidateArtifactIdentifier("test id", value); err == nil {
			t.Errorf("expected %q to be invalid", value)
		}
	}
}

func TestPathOperationsWithinRoot(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	dir := filepath.Join(root, "nested")
	path := filepath.Join(dir, "evidence.log")
	if err := MkdirAllWithin(root, dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := WriteFileWithin(root, path, []byte("evidence\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	data, err := ReadFileWithin(root, path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "evidence\n" {
		t.Fatalf("unexpected content %q", data)
	}
	info, err := StatWithin(root, path)
	if err != nil {
		t.Fatal(err)
	}
	if info.IsDir() || info.Size() != int64(len(data)) {
		t.Fatalf("unexpected file info %+v", info)
	}
	entries, err := ReadDirWithin(root, dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name() != "evidence.log" {
		t.Fatalf("unexpected directory entries %+v", entries)
	}
}

func TestMkdirWithinCreatesOnlyNewContainedDirectory(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	parent := filepath.Join(root, "runs")
	if err := os.Mkdir(parent, 0o755); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(parent, "run-001")
	if err := MkdirWithin(root, target, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := MkdirWithin(root, target, 0o755); !errors.Is(err, fs.ErrExist) {
		t.Fatalf("expected existing directory error, got %v", err)
	}
}

func TestOpenFileWithinStreamsToContainedPath(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	path := filepath.Join(root, "nested", "evidence.log")
	if err := MkdirAllWithin(root, filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	file, err := OpenFileWithin(root, path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := file.WriteString("partial evidence\n"); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "partial evidence\n" {
		t.Fatalf("unexpected content %q", data)
	}
}

func TestPathOperationsAllowInternalSymlink(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	targetDir := filepath.Join(root, "actual")
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		t.Fatal(err)
	}
	linkDir := filepath.Join(root, "linked")
	if err := os.Symlink(targetDir, linkDir); err != nil {
		t.Fatal(err)
	}
	linkedPath := filepath.Join(linkDir, "inside.log")
	if err := WriteFileWithin(root, linkedPath, []byte("inside\n"), 0o644); err != nil {
		t.Fatalf("expected internal symlink write to succeed: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(targetDir, "inside.log"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "inside\n" {
		t.Fatalf("unexpected content %q", data)
	}
}

func TestPathOperationsRejectSymlinkEscape(t *testing.T) {
	t.Parallel()
	t.Run("directory", func(t *testing.T) {
		t.Parallel()
		root := t.TempDir()
		external := t.TempDir()
		link := filepath.Join(root, "escape")
		if err := os.Symlink(external, link); err != nil {
			t.Fatal(err)
		}
		if err := MkdirAllWithin(root, filepath.Join(link, "nested"), 0o755); err == nil {
			t.Fatal("expected external directory symlink to fail closed")
		}
		if _, err := os.Stat(filepath.Join(external, "nested")); !os.IsNotExist(err) {
			t.Fatalf("expected no external directory, stat error=%v", err)
		}
	})

	t.Run("final file", func(t *testing.T) {
		t.Parallel()
		root := t.TempDir()
		externalPath := filepath.Join(t.TempDir(), "outside.log")
		if err := os.WriteFile(externalPath, []byte("unchanged\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		link := filepath.Join(root, "evidence.log")
		if err := os.Symlink(externalPath, link); err != nil {
			t.Fatal(err)
		}
		if err := WriteFileWithin(root, link, []byte("escaped\n"), 0o644); err == nil {
			t.Fatal("expected external file symlink to fail closed")
		}
		data, err := os.ReadFile(externalPath)
		if err != nil {
			t.Fatal(err)
		}
		if string(data) != "unchanged\n" {
			t.Fatalf("external file was modified: %q", data)
		}
	})

	t.Run("dangling", func(t *testing.T) {
		t.Parallel()
		root := t.TempDir()
		link := filepath.Join(root, "dangling.log")
		if err := os.Symlink(filepath.Join(root, "missing.log"), link); err != nil {
			t.Fatal(err)
		}
		if _, err := ReadFileWithin(root, link); err == nil {
			t.Fatal("expected dangling symlink to fail closed")
		}
	})
}
