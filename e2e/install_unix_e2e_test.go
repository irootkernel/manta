//go:build unix

package e2e

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestMakeInstallTargetsAndResolver(t *testing.T) {
	root := projectRoot(t)
	installRoot := t.TempDir()

	goBin := filepath.Join(installRoot, "gobin")
	installCmd := exec.Command("make", "install", "GOBIN="+goBin)
	installCmd.Dir = root
	runExpectedExit(t, installCmd, 0)
	versionOutput := strings.TrimSpace(runExpectedExit(t, exec.Command(filepath.Join(goBin, "kkachi-agent-tester"), "--version"), 0))
	version, ok := strings.CutPrefix(versionOutput, "kkachi-agent-tester v")
	if !ok || version == "" {
		t.Fatalf("unexpected installed version output %q", versionOutput)
	}

	toolchainRoot := filepath.Join(installRoot, "toolchains")
	toolchainCmd := exec.Command(
		"make",
		"install-toolchain",
		"BIN_DIR="+filepath.Join(installRoot, "build"),
		"TOOLCHAIN_ROOT="+toolchainRoot,
		"VERSION="+version,
	)
	toolchainCmd.Dir = root
	runExpectedExit(t, toolchainCmd, 0)

	toolchainBin := filepath.Join(toolchainRoot, "kat", "v"+version, "bin", "kkachi-agent-tester")
	resolverCmd := exec.Command(filepath.Join(root, "scripts", "kkachi-agent-tester-toolchain"), "--toolchain-status")
	resolverCmd.Dir = root
	resolverCmd.Env = append(withoutKATEnv(os.Environ(), root), "KKACHI_KAT_BIN="+toolchainBin)
	resolverOutput := runExpectedExit(t, resolverCmd, 0)
	for _, want := range []string{
		"kat_bin=" + toolchainBin,
		"kat_version_source=KKACHI_KAT_BIN",
		"kat_version_output=" + versionOutput,
	} {
		if !strings.Contains(resolverOutput, want) {
			t.Fatalf("resolver output missing %q:\n%s", want, resolverOutput)
		}
	}
}
