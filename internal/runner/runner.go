package runner

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"sync"
	"syscall"
	"time"

	"kkachi-agent-tester/internal/model"
)

type lockedBuffer struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (l *lockedBuffer) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.b.Write(p)
}

func (l *lockedBuffer) Bytes() []byte {
	l.mu.Lock()
	defer l.mu.Unlock()
	return append([]byte(nil), l.b.Bytes()...)
}

func Execute(ctx context.Context, workDir, commandID, lane, parser string, argv []string, timeoutSec int) (model.RunOutput, error) {
	if len(argv) == 0 {
		return model.RunOutput{}, model.NewKATError(model.ExitCodeConfigError, "execute command", fmt.Errorf("empty argv"))
	}
	started := time.Now().UTC()
	runCtx, cancel := context.WithTimeout(ctx, time.Duration(timeoutSec)*time.Second)
	defer cancel()

	cmd := exec.CommandContext(runCtx, argv[0], argv[1:]...)
	cmd.Dir = workDir
	var log lockedBuffer
	cmd.Stdout = &log
	cmd.Stderr = &log

	err := cmd.Run()
	ended := time.Now().UTC()
	output := model.RunOutput{
		Metadata: model.RunMetadata{
			CommandID:   commandID,
			Lane:        lane,
			Parser:      parser,
			CommandArgv: append([]string(nil), argv...),
			StartedAt:   started,
			EndedAt:     ended,
			DurationMS:  ended.Sub(started).Milliseconds(),
		},
		RawLogBytes: log.Bytes(),
	}

	if err == nil {
		output.Status = model.RunStatusPassed
		output.Metadata.ExitCode = 0
		return output, nil
	}

	if errors.Is(runCtx.Err(), context.DeadlineExceeded) {
		output.Status = model.RunStatusTimedOut
		output.TimedOut = true
		output.Metadata.ExitCode = int(model.ExitCodeTimeout)
		return output, nil
	}

	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		output.Metadata.ExitCode = exitErr.ExitCode()
		if ws, ok := exitErr.Sys().(syscall.WaitStatus); ok && ws.Signaled() {
			output.Status = model.RunStatusKilled
			output.Killed = true
		} else {
			output.Status = model.RunStatusFailed
		}
		return output, nil
	}

	if errors.Is(err, io.EOF) {
		output.Status = model.RunStatusKilled
		output.Killed = true
		return output, nil
	}

	return model.RunOutput{}, model.NewKATError(model.ExitCodeParserError, "execute command", err)
}
