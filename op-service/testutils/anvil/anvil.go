package anvil

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/ethereum/go-ethereum/log"
)

func Test(t *testing.T) {
	if os.Getenv("ENABLE_ANVIL") == "" {
		t.Skip("skipping Anvil test")
	}
}

type Runner struct {
	proc      *exec.Cmd
	stdout    io.ReadCloser
	stderr    io.ReadCloser
	logger    log.Logger
	startedCh chan struct{}
	wg        sync.WaitGroup
	port      int32
}

func New(l1RPCURL string, logger log.Logger) (*Runner, error) {
	proc := exec.Command(
		"anvil",
		"--fork-url", l1RPCURL,
		"--port",
		"0",
	)
	stdout, err := proc.StdoutPipe()
	if err != nil {
		return nil, err
	}
	stderr, err := proc.StderrPipe()
	if err != nil {
		return nil, err
	}

	return &Runner{
		proc:      proc,
		stdout:    stdout,
		stderr:    stderr,
		logger:    logger,
		startedCh: make(chan struct{}, 1),
	}, nil
}

func (r *Runner) Start(ctx context.Context) error {
	if err := r.proc.Start(); err != nil {
		return err
	}

	r.wg.Add(2)
	go r.outputStream(r.stdout)
	go r.outputStream(r.stderr)

	select {
	case <-r.startedCh:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (r *Runner) Stop() error {
	err := r.proc.Process.Signal(os.Interrupt)
	if err != nil {
		return err
	}

	// make sure the output streams close
	defer r.wg.Wait()
	return r.proc.Wait()
}

func (r *Runner) outputStream(stream io.ReadCloser) {
	defer r.wg.Done()
	scanner := bufio.NewScanner(stream)
	listenLine := "Listening on 127.0.0.1"

	for scanner.Scan() {
		line := scanner.Text()

		if strings.Contains(line, listenLine) && atomic.LoadInt32(&r.port) == 0 {
			split := strings.Split(line, ":")
			port, err := strconv.Atoi(strings.TrimSpace(split[len(split)-1]))
			if err == nil {
				atomic.StoreInt32(&r.port, int32(port))
				r.startedCh <- struct{}{}
			} else {
				r.logger.Error("failed to parse port from Anvil output", "err", err)
			}
		}

		r.logger.Debug("[ANVIL] " + scanner.Text())
	}
}

func (r *Runner) RPCUrl() string {
	port := atomic.LoadInt32(&r.port)
	if port == 0 {
		panic("anvil not started")
	}

	return fmt.Sprintf("http://localhost:%d", port)
}
