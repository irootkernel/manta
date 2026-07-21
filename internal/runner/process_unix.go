//go:build unix

package runner

import (
	"errors"
	"os"
	"os/exec"
	"syscall"
)

func handledSignals() []os.Signal {
	return []os.Signal{os.Interrupt, syscall.SIGTERM}
}

func prepareProcess(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

func signalProcess(cmd *exec.Cmd, sig os.Signal) error {
	signalValue, ok := sig.(syscall.Signal)
	if !ok {
		return cmd.Process.Signal(sig)
	}
	err := syscall.Kill(-cmd.Process.Pid, signalValue)
	if errors.Is(err, syscall.ESRCH) {
		return nil
	}
	return err
}

func killProcess(cmd *exec.Cmd) error {
	err := syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	if errors.Is(err, syscall.ESRCH) {
		return nil
	}
	return err
}

func cleanupProcessGroup(cmd *exec.Cmd) error {
	return killProcess(cmd)
}

func signalExitCode(sig os.Signal) int {
	if signalValue, ok := sig.(syscall.Signal); ok {
		return 128 + int(signalValue)
	}
	return 1
}
