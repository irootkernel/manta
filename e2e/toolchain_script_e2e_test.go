package e2e

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestToolchainScriptStatusWithEnvBinary(t *testing.T) {
	t.Parallel()
	python := requirePython3(t)
	root := projectRoot(t)
	repo := t.TempDir()
	canonicalRepo := canonicalPath(t, repo)
	bin := writeFakeKAT(t, t.TempDir(), "0.1.1")

	cmd := exec.Command(python, filepath.Join(root, "scripts", "kkachi-agent-tester-toolchain"), "--toolchain-status")
	cmd.Dir = repo
	cmd.Env = append(os.Environ(), "KKACHI_PROJECT_ROOT="+repo, "KKACHI_KAT_BIN="+bin)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("toolchain status failed: %v\n%s", err, string(out))
	}
	output := string(out)
	for _, want := range []string{
		"project_root=" + canonicalRepo,
		"kat_bin=" + bin,
		"kat_version_source=KKACHI_KAT_BIN",
		"kat_version_output=kkachi-agent-tester 0.1.1",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("status output missing %q:\n%s", want, output)
		}
	}
}

func TestToolchainScriptStatusWithVersionedMetadata(t *testing.T) {
	t.Parallel()
	python := requirePython3(t)
	root := projectRoot(t)
	repo := t.TempDir()
	toolchainRoot := t.TempDir()
	binDir := filepath.Join(toolchainRoot, "kat", "v0.1.1", "bin")
	_ = writeFakeKAT(t, binDir, "0.1.1")
	writeToolchainMetadata(t, repo, `schema_version: "kkachi.toolchain.v1"
kat:
  cli_version: "0.1.1"
`)

	cmd := exec.Command(python, filepath.Join(root, "scripts", "kkachi-agent-tester-toolchain"), "--toolchain-status")
	cmd.Dir = repo
	cmd.Env = append(os.Environ(), "KKACHI_PROJECT_ROOT="+repo, "KKACHI_TOOLCHAIN_ROOT="+toolchainRoot)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("toolchain status failed: %v\n%s", err, string(out))
	}
	output := string(out)
	for _, want := range []string{
		"kat_version_source=kat.cli_version",
		"kat_cli_version=v0.1.1",
		"kat_version_output=kkachi-agent-tester 0.1.1",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("status output missing %q:\n%s", want, output)
		}
	}
}

func TestToolchainScriptForwardsArguments(t *testing.T) {
	t.Parallel()
	python := requirePython3(t)
	root := projectRoot(t)
	repo := t.TempDir()
	canonicalRepo := canonicalPath(t, repo)
	bin := writeFakeKAT(t, t.TempDir(), "0.1.1")

	cmd := exec.Command(python, filepath.Join(root, "scripts", "kkachi-agent-tester-toolchain"), "run", "--lane", "unit", "--", "echo", "ok")
	cmd.Dir = repo
	cmd.Env = append(os.Environ(), "KKACHI_PROJECT_ROOT="+repo, "KKACHI_KAT_BIN="+bin)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("toolchain forwarding failed: %v\n%s", err, string(out))
	}
	output := string(out)
	for _, want := range []string{
		"argv=run --lane unit -- echo ok",
		"project_root=" + canonicalRepo,
		"effective_kat_bin=" + bin,
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("forwarded output missing %q:\n%s", want, output)
		}
	}
}

func TestToolchainScriptFailsClosed(t *testing.T) {
	t.Parallel()
	python := requirePython3(t)
	root := projectRoot(t)

	t.Run("missing source", func(t *testing.T) {
		t.Parallel()
		repo := t.TempDir()
		cmd := exec.Command(python, filepath.Join(root, "scripts", "kkachi-agent-tester-toolchain"), "--toolchain-status")
		cmd.Dir = repo
		cmd.Env = withoutKATEnv(os.Environ(), repo)
		out, err := cmd.CombinedOutput()
		if err == nil {
			t.Fatalf("expected missing source to fail, output=%s", string(out))
		}
		if !strings.Contains(string(out), "missing explicit KAT toolchain source") {
			t.Fatalf("expected missing-source diagnostic, got %s", string(out))
		}
	})

	t.Run("relative binary path", func(t *testing.T) {
		t.Parallel()
		repo := t.TempDir()
		writeToolchainMetadata(t, repo, `schema_version: "kkachi.toolchain.v1"
kat:
  binary_path: "bin/kkachi-agent-tester"
`)
		cmd := exec.Command(python, filepath.Join(root, "scripts", "kkachi-agent-tester-toolchain"), "--toolchain-status")
		cmd.Dir = repo
		cmd.Env = withoutKATEnv(os.Environ(), repo)
		out, err := cmd.CombinedOutput()
		if err == nil {
			t.Fatalf("expected relative path to fail, output=%s", string(out))
		}
		if !strings.Contains(string(out), "KAT must be an absolute path") {
			t.Fatalf("expected absolute-path diagnostic, got %s", string(out))
		}
	})

	t.Run("version mismatch", func(t *testing.T) {
		t.Parallel()
		repo := t.TempDir()
		bin := writeFakeKAT(t, t.TempDir(), "0.1.2")
		writeToolchainMetadata(t, repo, `schema_version: "kkachi.toolchain.v1"
kat:
  cli_version: "0.1.1"
  binary_path: "`+bin+`"
`)
		cmd := exec.Command(python, filepath.Join(root, "scripts", "kkachi-agent-tester-toolchain"), "--toolchain-status")
		cmd.Dir = repo
		cmd.Env = withoutKATEnv(os.Environ(), repo)
		out, err := cmd.CombinedOutput()
		if err == nil {
			t.Fatalf("expected version mismatch to fail, output=%s", string(out))
		}
		if !strings.Contains(string(out), "KAT binary version mismatch") {
			t.Fatalf("expected version mismatch diagnostic, got %s", string(out))
		}
	})
}

func requirePython3(t *testing.T) string {
	t.Helper()
	python, err := exec.LookPath("python3")
	if err != nil {
		t.Skip("python3 is required for toolchain script e2e tests")
	}
	return python
}

func writeFakeKAT(t *testing.T, dir string, version string) string {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	bin := filepath.Join(dir, "kkachi-agent-tester")
	body := strings.Join([]string{
		"#!/bin/sh",
		`if [ "${1:-}" = "--version" ]; then`,
		"  echo 'kkachi-agent-tester " + version + "'",
		"  exit 0",
		"fi",
		`printf 'argv=%s\n' "$*"`,
		`printf 'project_root=%s\n' "$KKACHI_PROJECT_ROOT"`,
		`printf 'effective_kat_bin=%s\n' "$KKACHI_EFFECTIVE_KAT_BIN"`,
	}, "\n") + "\n"
	if err := os.WriteFile(bin, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	return bin
}

func writeToolchainMetadata(t *testing.T, repo string, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(repo, ".kkachi"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, ".kkachi", "toolchain.yaml"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func withoutKATEnv(env []string, projectRoot string) []string {
	filtered := make([]string, 0, len(env)+1)
	for _, item := range env {
		if strings.HasPrefix(item, "KKACHI_KAT_BIN=") || strings.HasPrefix(item, "KKACHI_TOOLCHAIN_ROOT=") || strings.HasPrefix(item, "KKACHI_PROJECT_ROOT=") {
			continue
		}
		filtered = append(filtered, item)
	}
	return append(filtered, "KKACHI_PROJECT_ROOT="+projectRoot)
}

func canonicalPath(t *testing.T, path string) string {
	t.Helper()
	canonical, err := filepath.EvalSymlinks(path)
	if err != nil {
		t.Fatal(err)
	}
	return canonical
}

func TestToolchainScriptIsExecutable(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" {
		t.Skip("POSIX executable bit is not meaningful on windows")
	}
	root := projectRoot(t)
	info, err := os.Stat(filepath.Join(root, "scripts", "kkachi-agent-tester-toolchain"))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm()&0111 == 0 {
		t.Fatalf("expected script to be executable, mode=%v", info.Mode().Perm())
	}
}
