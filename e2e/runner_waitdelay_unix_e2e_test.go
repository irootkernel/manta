//go:build unix

package e2e

import (
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/SeventeenthEarth/kkachi-agent-tester/internal/model"
)

func TestBinaryRecoversSuccessfulLeaderAfterWaitDelay(t *testing.T) {
	t.Parallel()
	root := projectRoot(t)
	bin := buildBinary(t, root)
	repo := t.TempDir()
	script := "#!/bin/sh\nsh -c 'echo $$ > child.pid; sleep 30; echo late' &\nwhile [ ! -s child.pid ]; do sleep 0.01; done\necho started\n"
	writeE2EConfig(t, repo, script)

	cmd := exec.Command(bin, "--repo", repo, "--json", "run", "unit")
	cmd.Dir = repo
	output := runExpectedExit(t, cmd, 0)
	result := decodeBinaryRunResult(t, []byte(output))
	if result.Status != model.RunStatusPassed || result.ExitCode != 0 {
		t.Fatalf("unexpected run result: %+v", result)
	}
	summary, status, raw := loadBinaryRunArtifacts(t, repo, result)
	if summary.Status != model.RunStatusPassed || status.Status != model.RunStatusPassed || string(raw) != "started\n" {
		t.Fatalf("unexpected artifacts: summary=%+v status=%+v raw=%q", summary, status, raw)
	}
	requireProcessGone(t, filepath.Join(repo, "child.pid"))
}
