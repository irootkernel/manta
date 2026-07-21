package runner

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/SeventeenthEarth/kkachi-agent-tester/internal/model"
)

const interruptGracePeriod = 2 * time.Second

type streamCapture struct {
	mu  sync.Mutex
	raw io.Writer
	b   bytes.Buffer
}

func (c *streamCapture) Write(p []byte) (int, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	_, _ = c.b.Write(p)
	n, err := c.raw.Write(p)
	if err == nil && n != len(p) {
		err = io.ErrShortWrite
	}
	return n, err
}

func (c *streamCapture) Bytes() []byte {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.b.Bytes()
}

func Execute(ctx context.Context, workDir, commandID, lane, parser string, argv []string, timeoutSec int, raw io.Writer) (model.RunOutput, error) {
	interrupts := make(chan os.Signal, 2)
	signal.Notify(interrupts, handledSignals()...)
	defer signal.Stop(interrupts)
	return executeWithSignals(ctx, workDir, commandID, lane, parser, argv, timeoutSec, raw, interrupts, interruptGracePeriod)
}

func executeWithSignals(ctx context.Context, workDir, commandID, lane, parser string, argv []string, timeoutSec int, raw io.Writer, interrupts <-chan os.Signal, gracePeriod time.Duration) (model.RunOutput, error) {
	if len(argv) == 0 {
		return model.RunOutput{}, model.NewKATError(model.ExitCodeConfigError, "execute command", fmt.Errorf("empty argv"))
	}

	started := time.Now().UTC()
	runCtx, cancel := context.WithTimeout(ctx, time.Duration(timeoutSec)*time.Second)
	defer cancel()

	cmd := exec.Command(argv[0], argv[1:]...)
	cmd.Dir = workDir
	cmd.WaitDelay = gracePeriod
	prepareProcess(cmd)
	capture := &streamCapture{raw: raw}
	cmd.Stdout = capture
	cmd.Stderr = capture

	if err := cmd.Start(); err != nil {
		return model.RunOutput{}, model.NewKATError(model.ExitCodeParserError, "execute command", err)
	}
	waited := make(chan error, 1)
	go func() {
		waited <- cmd.Wait()
	}()

	select {
	case err := <-waited:
		_ = cleanupProcessGroup(cmd)
		if errors.Is(err, exec.ErrWaitDelay) && cmd.ProcessState != nil && cmd.ProcessState.Success() {
			err = nil
		}
		output := completedOutput(started, commandID, lane, parser, argv, capture)
		return classifyWait(output, err)
	case sig := <-interrupts:
		return finishInterrupted(started, commandID, lane, parser, argv, capture, cmd, waited, interrupts, sig, gracePeriod), nil
	case <-runCtx.Done():
		select {
		case sig := <-interrupts:
			return finishInterrupted(started, commandID, lane, parser, argv, capture, cmd, waited, interrupts, sig, gracePeriod), nil
		default:
		}
		_ = killProcess(cmd)
		<-waited
		output := completedOutput(started, commandID, lane, parser, argv, capture)
		if errors.Is(runCtx.Err(), context.DeadlineExceeded) {
			output.Status = model.RunStatusTimedOut
			output.Metadata.ExitCode = int(model.ExitCodeTimeout)
		} else {
			output.Status = model.RunStatusKilled
			output.Metadata.ExitCode = 137
		}
		return output, nil
	}
}

func finishInterrupted(started time.Time, commandID, lane, parser string, argv []string, capture *streamCapture, cmd *exec.Cmd, waited <-chan error, interrupts <-chan os.Signal, sig os.Signal, gracePeriod time.Duration) model.RunOutput {
	if err := signalProcess(cmd, sig); err != nil {
		_ = killProcess(cmd)
		<-waited
	} else {
		timer := time.NewTimer(gracePeriod)
		select {
		case <-waited:
			timer.Stop()
			// Wait reaps the command leader; its process group may still contain descendants.
			_ = killProcess(cmd)
		case <-interrupts:
			timer.Stop()
			_ = killProcess(cmd)
			<-waited
		case <-timer.C:
			_ = killProcess(cmd)
			<-waited
		}
	}
	output := completedOutput(started, commandID, lane, parser, argv, capture)
	output.Status = model.RunStatusKilled
	output.Metadata.ExitCode = signalExitCode(sig)
	return output
}

func completedOutput(started time.Time, commandID, lane, parser string, argv []string, capture *streamCapture) model.RunOutput {
	ended := time.Now().UTC()
	return model.RunOutput{
		Metadata: model.RunMetadata{
			CommandID:   commandID,
			Lane:        lane,
			Parser:      parser,
			CommandArgv: append([]string(nil), argv...),
			StartedAt:   started,
			EndedAt:     ended,
			DurationMS:  ended.Sub(started).Milliseconds(),
		},
		RawLogBytes: capture.Bytes(),
	}
}

func classifyWait(output model.RunOutput, err error) (model.RunOutput, error) {
	if err == nil {
		output.Status = model.RunStatusPassed
		output.Metadata.ExitCode = 0
		return output, nil
	}

	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		if ws, ok := exitErr.Sys().(syscall.WaitStatus); ok && ws.Signaled() {
			output.Status = model.RunStatusKilled
			output.Metadata.ExitCode = signalExitCode(ws.Signal())
		} else {
			output.Status = model.RunStatusFailed
			output.Metadata.ExitCode = exitErr.ExitCode()
		}
		return output, nil
	}

	return model.RunOutput{}, model.NewKATError(model.ExitCodeParserError, "execute command", err)
}
