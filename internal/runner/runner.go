// Package runner starts external commands (never through a shell), streams their output line by
// line, and kills the whole process group on cancel or timeout.
package runner

import (
	"bufio"
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
	"syscall"
	"time"
)

// Cmd describes one external command.
type Cmd struct {
	Name    string
	Args    []string
	Dir     string
	Env     []string // nil = inherit
	Stdin   string
	Timeout time.Duration // 0 = none
}

// Result is the outcome of a command that ran to completion.
type Result struct {
	ExitCode int
	Output   string // stdout and stderr, interleaved
}

// Runner runs commands. onLine (may be nil) receives every output line.
type Runner interface {
	Run(ctx context.Context, c Cmd, onLine func(string)) (Result, error)
}

var (
	ErrTimeout  = errors.New("timeout")
	ErrCanceled = errors.New("canceled")
)

// ExecRunner is the real Runner.
type ExecRunner struct{}

const killGrace = 3 * time.Second

// Run starts c in its own process group and waits for it.
func (ExecRunner) Run(ctx context.Context, c Cmd, onLine func(string)) (Result, error) {
	cmd := exec.Command(c.Name, c.Args...)
	cmd.Dir = c.Dir
	cmd.Env = c.Env
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if c.Stdin != "" {
		cmd.Stdin = strings.NewReader(c.Stdin)
	}
	pr, pw, err := os.Pipe()
	if err != nil {
		return Result{}, err
	}
	cmd.Stdout, cmd.Stderr = pw, pw
	if err := cmd.Start(); err != nil {
		pr.Close()
		pw.Close()
		return Result{}, err
	}
	pw.Close()
	pgid := cmd.Process.Pid

	var out strings.Builder
	var mu sync.Mutex
	readDone := make(chan struct{})
	go func() {
		defer close(readDone)
		readLines(pr, func(l string) {
			mu.Lock()
			out.WriteString(l)
			out.WriteByte('\n')
			mu.Unlock()
			if onLine != nil {
				onLine(l)
			}
		})
	}()

	waitDone := make(chan error, 1)
	go func() { waitDone <- cmd.Wait() }()

	var timeout <-chan time.Time
	if c.Timeout > 0 {
		t := time.NewTimer(c.Timeout)
		defer t.Stop()
		timeout = t.C
	}

	var stopErr error
	var waitErr error
	select {
	case waitErr = <-waitDone:
	case <-ctx.Done():
		stopErr = ErrCanceled
	case <-timeout:
		stopErr = ErrTimeout
	}
	if stopErr != nil {
		_ = syscall.Kill(-pgid, syscall.SIGTERM)
		select {
		case waitErr = <-waitDone:
		case <-time.After(killGrace):
			_ = syscall.Kill(-pgid, syscall.SIGKILL)
			waitErr = <-waitDone
		}
	}
	// Leftover children (e.g. ffmpeg spawned by python) must not outlive the command.
	_ = syscall.Kill(-pgid, syscall.SIGKILL)
	<-readDone
	pr.Close()

	mu.Lock()
	res := Result{Output: out.String(), ExitCode: cmd.ProcessState.ExitCode()}
	mu.Unlock()
	if stopErr != nil {
		return res, stopErr
	}
	var exitErr *exec.ExitError
	if waitErr != nil && !errors.As(waitErr, &exitErr) {
		return res, waitErr
	}
	return res, nil
}

func readLines(r io.Reader, fn func(string)) {
	br := bufio.NewReaderSize(r, 64*1024)
	for {
		line, err := br.ReadString('\n')
		if line != "" {
			fn(strings.TrimRight(line, "\r\n"))
		}
		if err != nil {
			return
		}
	}
}
