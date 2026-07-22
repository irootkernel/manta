package safety

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReadFileLimitedEnforcesExactByteLimit(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	for _, test := range []struct {
		name    string
		size    int
		wantErr bool
	}{
		{name: "exact limit", size: MaxConfigRuleInputBytes},
		{name: "one byte over", size: MaxConfigRuleInputBytes + 1, wantErr: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			path := filepath.Join(dir, "input")
			if err := os.WriteFile(path, bytes.Repeat([]byte("x"), test.size), 0o644); err != nil {
				t.Fatal(err)
			}
			data, err := ReadFileLimited(path)
			if test.wantErr {
				if err == nil || !IsInputTooLarge(err) || !strings.Contains(err.Error(), "input exceeds 262144 bytes") {
					t.Fatalf("expected size error, got data=%d err=%v", len(data), err)
				}
				return
			}
			if err != nil || len(data) != test.size {
				t.Fatalf("ReadFileLimited() data=%d err=%v", len(data), err)
			}
		})
	}
	if IsInputTooLarge(os.ErrNotExist) {
		t.Fatal("ordinary file error was classified as a size error")
	}
}

func TestReadFileWithinLimitEnforcesExactByteLimit(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	exactPath := filepath.Join(root, "exact.input")
	oversizedPath := filepath.Join(root, "oversized.input")
	if err := os.WriteFile(exactPath, bytes.Repeat([]byte("x"), MaxConfigRuleInputBytes), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(oversizedPath, bytes.Repeat([]byte("x"), MaxConfigRuleInputBytes+1), 0o644); err != nil {
		t.Fatal(err)
	}
	data, err := ReadFileWithinLimit(root, exactPath)
	if err != nil || len(data) != MaxConfigRuleInputBytes {
		t.Fatalf("exact read data=%d err=%v", len(data), err)
	}
	if _, err := ReadFileWithinLimit(root, oversizedPath); err == nil || !strings.Contains(err.Error(), "input exceeds 262144 bytes") {
		t.Fatalf("expected oversized contained read to fail, got %v", err)
	}
}

func TestReadFileWithinLimitRejectsSymlinkEscape(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	externalPath := filepath.Join(t.TempDir(), "outside.input")
	if err := os.WriteFile(externalPath, []byte("outside"), 0o644); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, "escape.input")
	if err := os.Symlink(externalPath, link); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadFileWithinLimit(root, link); err == nil {
		t.Fatal("expected symlink escape to fail closed")
	}
}
