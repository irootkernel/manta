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
	versionOutput := strings.TrimSpace(runExpectedExit(t, exec.Command(filepath.Join(goBin, "manta"), "--version"), 0))
	version, ok := strings.CutPrefix(versionOutput, "manta v")
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

	toolchainBin := filepath.Join(toolchainRoot, "v"+version, "bin", "manta")
	resolverCmd := exec.Command(filepath.Join(root, "scripts", "manta-toolchain"), "--toolchain-status")
	resolverCmd.Dir = root
	resolverCmd.Env = append(withoutMantaEnv(os.Environ(), root), "MANTA_BIN="+toolchainBin)
	resolverOutput := runExpectedExit(t, resolverCmd, 0)
	for _, want := range []string{
		"manta_bin=" + toolchainBin,
		"manta_version_source=MANTA_BIN",
		"manta_version_output=" + versionOutput,
	} {
		if !strings.Contains(resolverOutput, want) {
			t.Fatalf("resolver output missing %q:\n%s", want, resolverOutput)
		}
	}
}
