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
	bin := writeFakeManta(t, t.TempDir(), "0.1.4")
	writeToolchainMetadata(t, repo, `schema_version: "manta.toolchain.v1"
manta:
  cli_version: "9.9.9"
  binary_path: "relative-metadata-binary"
`)

	out, err := runToolchainScript(python, root, repo, append(os.Environ(), "MANTA_PROJECT_ROOT="+repo, "MANTA_BIN="+bin), "--toolchain-status")
	if err != nil {
		t.Fatalf("toolchain status failed: %v\n%s", err, string(out))
	}
	output := string(out)
	for _, want := range []string{
		"project_root=" + canonicalRepo,
		"manta_bin=" + bin,
		"manta_version_source=MANTA_BIN",
		"manta_version_output=manta 0.1.4",
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
	binDir := filepath.Join(toolchainRoot, "v0.1.4", "bin")
	_ = writeFakeManta(t, binDir, "0.1.4")
	writeToolchainMetadata(t, repo, `schema_version: "manta.toolchain.v1"
manta:
  cli_version: "0.1.4"
`)

	out, err := runToolchainScript(python, root, repo, append(os.Environ(), "MANTA_PROJECT_ROOT="+repo, "MANTA_TOOLCHAIN_ROOT="+toolchainRoot), "--toolchain-status")
	if err != nil {
		t.Fatalf("toolchain status failed: %v\n%s", err, string(out))
	}
	output := string(out)
	for _, want := range []string{
		"manta_version_source=manta.cli_version",
		"manta_cli_version=v0.1.4",
		"manta_version_output=manta 0.1.4",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("status output missing %q:\n%s", want, output)
		}
	}
}

func TestToolchainScriptStatusWithAbsoluteBinaryMetadata(t *testing.T) {
	t.Parallel()
	python := requirePython3(t)
	root := projectRoot(t)
	repo := t.TempDir()
	bin := writeFakeManta(t, t.TempDir(), "0.1.4")
	writeToolchainMetadata(t, repo, `schema_version: "manta.toolchain.v1"
manta:
  cli_version: "0.1.4"
  binary_path: "`+bin+`"
`)

	out, err := runToolchainScript(python, root, repo, withoutMantaEnv(os.Environ(), repo), "--toolchain-status")
	if err != nil {
		t.Fatalf("toolchain status failed: %v\n%s", err, string(out))
	}
	output := string(out)
	for _, want := range []string{
		"manta_bin=" + bin,
		"manta_version_source=manta.binary_path",
		"manta_cli_version=v0.1.4",
		"manta_version_output=manta 0.1.4",
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
	bin := writeFakeManta(t, t.TempDir(), "0.1.4")

	out, err := runToolchainScript(python, root, repo, append(os.Environ(), "MANTA_PROJECT_ROOT="+repo, "MANTA_BIN="+bin), "run", "--lane", "unit", "--", "echo", "ok")
	if err != nil {
		t.Fatalf("toolchain forwarding failed: %v\n%s", err, string(out))
	}
	output := string(out)
	for _, want := range []string{
		"argv=run --lane unit -- echo ok",
		"project_root=" + canonicalRepo,
		"effective_manta_bin=" + bin,
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
		out, err := runToolchainScript(python, root, repo, withoutMantaEnv(os.Environ(), repo), "--toolchain-status")
		if err == nil {
			t.Fatalf("expected missing source to fail, output=%s", string(out))
		}
		if !strings.Contains(string(out), "missing explicit Manta toolchain source") {
			t.Fatalf("expected missing-source diagnostic, got %s", string(out))
		}
	})

	t.Run("relative binary path", func(t *testing.T) {
		t.Parallel()
		repo := t.TempDir()
		writeToolchainMetadata(t, repo, `schema_version: "manta.toolchain.v1"
manta:
  binary_path: "bin/manta"
`)
		out, err := runToolchainScript(python, root, repo, withoutMantaEnv(os.Environ(), repo), "--toolchain-status")
		if err == nil {
			t.Fatalf("expected relative path to fail, output=%s", string(out))
		}
		if !strings.Contains(string(out), "Manta must be an absolute path") {
			t.Fatalf("expected absolute-path diagnostic, got %s", string(out))
		}
	})

	t.Run("version mismatch", func(t *testing.T) {
		t.Parallel()
		repo := t.TempDir()
		bin := writeFakeManta(t, t.TempDir(), "0.1.2")
		writeToolchainMetadata(t, repo, `schema_version: "manta.toolchain.v1"
manta:
  cli_version: "0.1.4"
  binary_path: "`+bin+`"
`)
		out, err := runToolchainScript(python, root, repo, withoutMantaEnv(os.Environ(), repo), "--toolchain-status")
		if err == nil {
			t.Fatalf("expected version mismatch to fail, output=%s", string(out))
		}
		if !strings.Contains(string(out), "Manta binary version mismatch") {
			t.Fatalf("expected version mismatch diagnostic, got %s", string(out))
		}
	})

	t.Run("non-executable binary", func(t *testing.T) {
		t.Parallel()
		repo := t.TempDir()
		bin := writeFakeManta(t, t.TempDir(), "0.1.4")
		if err := os.Chmod(bin, 0o644); err != nil {
			t.Fatal(err)
		}
		writeToolchainMetadata(t, repo, `schema_version: "manta.toolchain.v1"
manta:
  binary_path: "`+bin+`"
`)
		out, err := runToolchainScript(python, root, repo, withoutMantaEnv(os.Environ(), repo), "--toolchain-status")
		if err == nil {
			t.Fatalf("expected non-executable binary to fail, output=%s", string(out))
		}
		if !strings.Contains(string(out), "binary is not executable") {
			t.Fatalf("expected executable diagnostic, got %s", string(out))
		}
	})

	t.Run("malformed version", func(t *testing.T) {
		t.Parallel()
		repo := t.TempDir()
		writeToolchainMetadata(t, repo, `schema_version: "manta.toolchain.v1"
manta:
  cli_version: "0.1"
`)
		out, err := runToolchainScript(python, root, repo, withoutMantaEnv(os.Environ(), repo), "--toolchain-status")
		if err == nil {
			t.Fatalf("expected malformed version to fail, output=%s", string(out))
		}
		if !strings.Contains(string(out), "invalid manta.cli_version") {
			t.Fatalf("expected malformed-version diagnostic, got %s", string(out))
		}
	})

	t.Run("unsupported schema", func(t *testing.T) {
		t.Parallel()
		repo := t.TempDir()
		writeToolchainMetadata(t, repo, `schema_version: "manta.toolchain.v2"
manta:
  cli_version: "0.1.4"
`)
		out, err := runToolchainScript(python, root, repo, withoutMantaEnv(os.Environ(), repo), "--toolchain-status")
		if err == nil {
			t.Fatalf("expected unsupported schema to fail, output=%s", string(out))
		}
		if !strings.Contains(string(out), "unsupported schema_version") {
			t.Fatalf("expected schema diagnostic, got %s", string(out))
		}
	})
}

func runToolchainScript(python, root, repo string, env []string, args ...string) ([]byte, error) {
	commandArgs := append([]string{filepath.Join(root, "scripts", "manta-toolchain")}, args...)
	cmd := exec.Command(python, commandArgs...)
	cmd.Dir = repo
	cmd.Env = env
	return cmd.CombinedOutput()
}

func requirePython3(t *testing.T) string {
	t.Helper()
	python, err := exec.LookPath("python3")
	if err != nil {
		t.Skip("python3 is required for toolchain script e2e tests")
	}
	return python
}

func writeFakeManta(t *testing.T, dir string, version string) string {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	bin := filepath.Join(dir, "manta")
	body := strings.Join([]string{
		"#!/bin/sh",
		`if [ "${1:-}" = "--version" ]; then`,
		"  echo 'manta " + version + "'",
		"  exit 0",
		"fi",
		`printf 'argv=%s\n' "$*"`,
		`printf 'project_root=%s\n' "$MANTA_PROJECT_ROOT"`,
		`printf 'effective_manta_bin=%s\n' "$MANTA_EFFECTIVE_BIN"`,
	}, "\n") + "\n"
	if err := os.WriteFile(bin, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	return bin
}

func writeToolchainMetadata(t *testing.T, repo string, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(repo, ".manta"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, ".manta", "toolchain.yaml"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func withoutMantaEnv(env []string, projectRoot string) []string {
	filtered := make([]string, 0, len(env)+1)
	for _, item := range env {
		if strings.HasPrefix(item, "MANTA_BIN=") || strings.HasPrefix(item, "MANTA_TOOLCHAIN_ROOT=") || strings.HasPrefix(item, "MANTA_PROJECT_ROOT=") {
			continue
		}
		filtered = append(filtered, item)
	}
	return append(filtered, "MANTA_PROJECT_ROOT="+projectRoot)
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
	info, err := os.Stat(filepath.Join(root, "scripts", "manta-toolchain"))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm()&0111 == 0 {
		t.Fatalf("expected script to be executable, mode=%v", info.Mode().Perm())
	}
}
