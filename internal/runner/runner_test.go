package runner

import (
	"context"
	"errors"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"
)

func fake(name string) string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(file), "..", "..", "testdata", "fakes", name)
}

func TestRunStreamsLines(t *testing.T) {
	var mu sync.Mutex
	var got []string
	res, err := ExecRunner{}.Run(context.Background(), Cmd{Name: fake("lines.sh")}, func(l string) {
		mu.Lock()
		got = append(got, l)
		mu.Unlock()
	})
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(got)
	if strings.Join(got, ",") != "a,b,c" || res.ExitCode != 3 {
		t.Fatalf("lines %v exit %d", got, res.ExitCode)
	}
	if !strings.Contains(res.Output, "a\n") || !strings.Contains(res.Output, "c") {
		t.Fatalf("output %q", res.Output)
	}
}

func TestStdin(t *testing.T) {
	res, err := ExecRunner{}.Run(context.Background(), Cmd{Name: "/bin/cat", Stdin: "xin chào"}, nil)
	if err != nil || strings.TrimSpace(res.Output) != "xin chào" {
		t.Fatalf("%q %v", res.Output, err)
	}
}

func TestDirIsUsed(t *testing.T) {
	dir := t.TempDir()
	res, err := ExecRunner{}.Run(context.Background(), Cmd{Name: "/bin/pwd", Dir: dir}, nil)
	real, _ := filepath.EvalSymlinks(dir)
	got, _ := filepath.EvalSymlinks(strings.TrimSpace(res.Output))
	if err != nil || got != real {
		t.Fatalf("%q vs %q %v", res.Output, real, err)
	}
}

func runSleeper(t *testing.T, ctx context.Context, c Cmd, cancel func()) (int, error) {
	t.Helper()
	child := 0
	_, err := ExecRunner{}.Run(ctx, c, func(l string) {
		if strings.HasPrefix(l, "child=") {
			child, _ = strconv.Atoi(strings.TrimPrefix(l, "child="))
		}
		if l == "start" && cancel != nil {
			cancel()
		}
	})
	return child, err
}

func assertDead(t *testing.T, pid int) {
	t.Helper()
	if pid == 0 {
		t.Fatal("child pid not seen")
	}
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if syscall.Kill(pid, 0) != nil {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	syscall.Kill(pid, syscall.SIGKILL)
	t.Fatalf("grandchild %d still alive", pid)
}

func TestCancelKillsGroup(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	start := time.Now()
	child, err := runSleeper(t, ctx, Cmd{Name: fake("sleeper.sh")}, cancel)
	if !errors.Is(err, ErrCanceled) {
		t.Fatalf("want ErrCanceled, got %v", err)
	}
	if time.Since(start) > 5*time.Second {
		t.Fatal("cancel too slow")
	}
	assertDead(t, child)
}

func TestTimeout(t *testing.T) {
	child, err := runSleeper(t, context.Background(), Cmd{Name: fake("sleeper.sh"), Timeout: 300 * time.Millisecond}, nil)
	if !errors.Is(err, ErrTimeout) {
		t.Fatalf("want ErrTimeout, got %v", err)
	}
	assertDead(t, child)
}

func TestMissingBinary(t *testing.T) {
	_, err := ExecRunner{}.Run(context.Background(), Cmd{Name: "/nope/missing"}, nil)
	if err == nil {
		t.Fatal("expected error")
	}
}

func envVal(env []string, key string) (string, int) {
	val, n := "", 0
	for _, e := range env {
		if strings.HasPrefix(e, key+"=") {
			val = strings.TrimPrefix(e, key+"=")
			n++
		}
	}
	return val, n
}

func TestChildEnvPath(t *testing.T) {
	env := ChildEnv([]string{"PATH=/usr/bin:/usr/local/bin:/bin", "HOME=/h"}, "/Users/x/.nvm/v/bin")
	p, n := envVal(env, "PATH")
	if n != 1 || p != "/Users/x/.nvm/v/bin:/opt/homebrew/bin:/usr/local/bin:/usr/bin:/bin" {
		t.Fatalf("PATH=%q (%d)", p, n)
	}
	if v, _ := envVal(env, "HOME"); v != "/h" {
		t.Fatal("HOME lost")
	}
	if v, _ := envVal(env, "PYTHONUNBUFFERED"); v != "1" {
		t.Fatal("PYTHONUNBUFFERED")
	}
	env = ChildEnv([]string{"HOME=/h", "LANG=vi_VN.UTF-8"})
	p, _ = envVal(env, "PATH")
	if p != "/opt/homebrew/bin:/usr/local/bin" {
		t.Fatalf("no-PATH base: %q", p)
	}
	if v, _ := envVal(env, "LANG"); v != "vi_VN.UTF-8" {
		t.Fatal("LANG overridden")
	}
}
