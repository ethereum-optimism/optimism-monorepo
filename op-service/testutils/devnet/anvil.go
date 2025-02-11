package devnet

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ethereum/go-ethereum/log"
)

type Anvil struct {
	args      map[string]string
	proc      *exec.Cmd
	stdout    io.ReadCloser
	stderr    io.ReadCloser
	logger    log.Logger
	startedCh chan struct{}
	wg        sync.WaitGroup
	port      int32
}

type AnvilOption func(*Anvil)

func WithForkURL(url string) AnvilOption {
	return func(a *Anvil) {
		a.args["--fork-url"] = url
	}
}

func WithBaseFee(baseFee uint64) AnvilOption {
	return func(a *Anvil) {
		a.args["--base-fee"] = strconv.FormatUint(baseFee, 10)
	}
}

func WithBlockTime(bt int) AnvilOption {
	if bt < 0 {
		panic("block time must be non-negative")
	}
	return func(a *Anvil) {
		a.args["--block-time"] = strconv.Itoa(bt)
	}
}

func WithChainID(id uint64) AnvilOption {
	return func(a *Anvil) {
		a.args["--chain-id"] = strconv.FormatUint(id, 10)
	}
}

func NewAnvil(logger log.Logger, opts ...AnvilOption) (*Anvil, error) {
	if _, err := exec.LookPath("anvil"); err != nil {
		return nil, fmt.Errorf("anvil not found in PATH: %w", err)
	}

	a := &Anvil{
		args: map[string]string{
			"--base-fee": "1000000000",
			"--port":     "0",
		},
		logger:    logger,
		startedCh: make(chan struct{}, 1),
	}
	for _, opt := range opts {
		opt(a)
	}

	return a, nil
}

func (r *Anvil) Start() error {
	var args []string
	for k, v := range r.args {
		args = append(args, k, v)
	}
	proc := exec.Command("anvil", args...)
	stdout, err := proc.StdoutPipe()
	if err != nil {
		return err
	}
	stderr, err := proc.StderrPipe()
	if err != nil {
		return err
	}

	r.proc = proc
	r.stdout = stdout
	r.stderr = stderr

	if err := r.proc.Start(); err != nil {
		return err
	}

	r.wg.Add(2)
	go r.outputStream(r.stdout)
	go r.outputStream(r.stderr)

	timeoutC := time.NewTimer(5 * time.Second)

	select {
	case <-r.startedCh:
		return nil
	case <-timeoutC.C:
		_ = r.Stop()
		return fmt.Errorf("anvil did not start in time")
	}
}

func (r *Anvil) Stop() error {
	if r.proc == nil {
		return nil
	}

	err := r.proc.Process.Signal(os.Interrupt)
	if err != nil {
		return err
	}

	// make sure the output streams close
	defer r.wg.Wait()
	return r.proc.Wait()
}

func (r *Anvil) outputStream(stream io.ReadCloser) {
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

		if atomic.LoadInt32(&r.port) == 0 {
			r.logger.Debug("[ANVIL] " + line)
		} else {
			r.logger.Trace("[ANVIL] " + line)
		}
	}
}

func (r *Anvil) RPCUrl() string {
	port := atomic.LoadInt32(&r.port)
	if port == 0 {
		panic("anvil not started")
	}

	return fmt.Sprintf("http://localhost:%d", port)
}
