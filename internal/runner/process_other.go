//go:build !unix

package runner

import (
	"os"
	"os/exec"
)

func handledSignals() []os.Signal {
	return []os.Signal{os.Interrupt}
}

func prepareProcess(_ *exec.Cmd) {}

func signalProcess(cmd *exec.Cmd, sig os.Signal) error {
	return cmd.Process.Signal(sig)
}

func killProcess(cmd *exec.Cmd) error {
	return cmd.Process.Kill()
}

func cleanupProcessGroup(_ *exec.Cmd) error {
	return nil
}

func signalExitCode(sig os.Signal) int {
	if sig == os.Interrupt {
		return 130
	}
	return 1
}
