// Copyright 2019 The go-ethereum Authors
// This file is part of the go-ethereum library.
//
// The go-ethereum library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-ethereum library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-ethereum library. If not, see <http://www.gnu.org/licenses/>.

// Package testlog provides a log handler for unit tests.
package testlog

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"os"
	"path"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"

	oplog "github.com/ethereum-optimism/optimism/op-service/log"
	"github.com/ethereum/go-ethereum/log"
)

var useColorInTestLog bool = true

func init() {
	if os.Getenv("OP_TESTLOG_DISABLE_COLOR") == "true" {
		useColorInTestLog = false
	}
}

// Testing interface to log to. Some functions are marked as Helper function to log the call site accurately.
// Standard Go testing.TB implements this, as well as Hive and other Go-like test frameworks.
type Testing interface {
	Logf(format string, args ...any)
	Helper()
	Name() string
	Cleanup(func())
}

// logger implements log.Logger such that all output goes to the unit test log via
// t.Logf(). All methods in between logger.Trace, logger.Debug, etc. are marked as test
// helpers, so the file and line number in unit test output correspond to the call site
// which emitted the log message.
type logger struct {
	t   Testing
	l   log.Logger
	mu  *sync.Mutex
	buf *bytes.Buffer
}

// Logger returns a logger which logs to the unit test log of t.
func Logger(t Testing, level slog.Level) log.Logger {
	return LoggerWithHandlerMod(t, level, func(h slog.Handler) slog.Handler { return h })
}

func LoggerWithHandlerMod(t Testing, level slog.Level, handlerMod func(slog.Handler) slog.Handler) log.Logger {
	l := &logger{t: t, mu: new(sync.Mutex), buf: new(bytes.Buffer)}

	var handler slog.Handler
	if outdir := os.Getenv("OP_TESTLOG_FILE_LOGGER_OUTDIR"); outdir != "" {
		handler = fileHandler(t, outdir, level)
	}

	// Check if handler is nil here because setupFileLogger will return nil if it fails to
	// create the logfile.
	if handler == nil {
		handler = log.NewTerminalHandlerWithLevel(l.buf, level, useColorInTestLog)
	}

	handler = handlerMod(handler)
	l.l = log.NewLogger(handler)

	return l
}

var (
	alnumRegexp = regexp.MustCompile(`[^a-zA-Z0-9]+`)
	flMtx       sync.Mutex
	flHandlers  = make(map[string]slog.Handler)
	rootSetup   sync.Once
)

func fileHandler(t Testing, outdir string, level slog.Level) slog.Handler {
	rootSetup.Do(func() {
		f, err := os.OpenFile(path.Join(outdir, fmt.Sprintf("root-%d.log", os.Getpid())), os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
		if err != nil {
			t.Logf("failed to open root log file: %v", err)
			return
		}

		rootHdlr := log.NewTerminalHandlerWithLevel(bufio.NewWriter(f), level, false)
		oplog.SetGlobalLogHandler(rootHdlr)
		t.Logf("redirecting root logger to %s", f.Name())
	})

	testName := fmt.Sprintf(
		"%s-%d.log",
		alnumRegexp.ReplaceAllString(strings.ReplaceAll(t.Name(), "/", "-"), ""),
		os.Getpid(),
	)

	flMtx.Lock()
	defer flMtx.Unlock()

	if h, ok := flHandlers[testName]; ok {
		return h
	}

	logPath := path.Join(outdir, testName)
	dw := newDeferredWriter(logPath)
	t.Cleanup(func() {
		if err := dw.Close(); err != nil {
			t.Logf("failed to close log file %s: %v", logPath, err)
		}

		flMtx.Lock()
		delete(flHandlers, testName)
		flMtx.Unlock()
	})
	t.Logf("writing test log to %s", logPath)
	h := log.NewTerminalHandlerWithLevel(dw, level, false)
	flHandlers[testName] = h
	return h
}

func (l *logger) Handler() slog.Handler {
	return l.l.Handler()
}

func (l *logger) Trace(msg string, ctx ...any) {
	l.t.Helper()
	l.mu.Lock()
	defer l.mu.Unlock()
	l.l.Trace(msg, ctx...)
	l.flush()
}

func (l *logger) Debug(msg string, ctx ...any) {
	l.t.Helper()
	l.mu.Lock()
	defer l.mu.Unlock()
	l.l.Debug(msg, ctx...)
	l.flush()
}

func (l *logger) Info(msg string, ctx ...any) {
	l.t.Helper()
	l.mu.Lock()
	defer l.mu.Unlock()
	l.l.Info(msg, ctx...)
	l.flush()
}

func (l *logger) Warn(msg string, ctx ...any) {
	l.t.Helper()
	l.mu.Lock()
	defer l.mu.Unlock()
	l.l.Warn(msg, ctx...)
	l.flush()
}

func (l *logger) Error(msg string, ctx ...any) {
	l.t.Helper()
	l.mu.Lock()
	defer l.mu.Unlock()
	l.l.Error(msg, ctx...)
	l.flush()
}

func (l *logger) Crit(msg string, ctx ...any) {
	l.t.Helper()
	l.mu.Lock()
	defer l.mu.Unlock()
	l.l.Crit(msg, ctx...)
	l.flush()
}

func (l *logger) Log(level slog.Level, msg string, ctx ...any) {
	l.t.Helper()
	l.mu.Lock()
	defer l.mu.Unlock()
	l.l.Log(level, msg, ctx...)
	l.flush()
}

func (l *logger) Write(level slog.Level, msg string, ctx ...any) {
	l.Log(level, msg, ctx...)
}

func (l *logger) New(ctx ...any) log.Logger {
	return &logger{l.t, l.l.New(ctx...), l.mu, l.buf}
}

func (l *logger) With(ctx ...any) log.Logger {
	return l.New(ctx...)
}

func (l *logger) Enabled(ctx context.Context, level slog.Level) bool {
	return l.l.Enabled(ctx, level)
}

// flush writes all buffered messages and clears the buffer.
func (l *logger) flush() {
	l.t.Helper()
	// 2 frame skip for flush() + public logger fn
	decorationLen := estimateInfoLen(2)
	padding := 0
	padLength := 30
	if decorationLen <= padLength {
		padding = padLength - decorationLen
	}

	scanner := bufio.NewScanner(l.buf)
	for scanner.Scan() {
		l.internalFlush("%*s%s", padding, "", scanner.Text())
	}
	l.buf.Reset()
}

func (l *logger) internalFlush(format string, args ...any) {
	defer func() {
		if r := recover(); r != nil {
			log.Warn("testlog: panic during flush", "recover", r)
		}
	}()

	l.t.Helper()
	l.t.Logf(format, args...)
}

// The Go testing lib uses the runtime package to get info about the calling site, and then decorates the line.
// We can't disable this decoration, but we can adjust the contents to align by padding after the info.
// To pad the right amount, we estimate how long the info is.
func estimateInfoLen(frameSkip int) int {
	var pc [50]uintptr
	// Skip two extra frames to account for this function
	// and runtime.Callers itself.
	n := runtime.Callers(frameSkip+2, pc[:])
	if n == 0 {
		return 8
	}
	frames := runtime.CallersFrames(pc[:n])
	frame, _ := frames.Next()
	file := frame.File
	line := frame.Line
	if file != "" {
		// Truncate file name at last file name separator.
		if index := strings.LastIndex(file, "/"); index >= 0 {
			file = file[index+1:]
		} else if index = strings.LastIndex(file, "\\"); index >= 0 {
			file = file[index+1:]
		}
		return 4 + len(file) + 1 + len(strconv.FormatInt(int64(line), 10))
	} else {
		return 8
	}
}

type deferredWriter struct {
	name  string
	w     *bufio.Writer
	close func() error
	mtx   sync.Mutex
}

func newDeferredWriter(name string) *deferredWriter {
	return &deferredWriter{name: name}
}

func (w *deferredWriter) Write(p []byte) (n int, err error) {
	w.mtx.Lock()
	defer w.mtx.Unlock()

	if w.w == nil {
		f, err := os.OpenFile(w.name, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			return 0, err
		}
		w.w = bufio.NewWriter(f)
		w.close = f.Close
	}

	return w.w.Write(p)
}

func (w *deferredWriter) Close() error {
	w.mtx.Lock()
	defer w.mtx.Unlock()
	if w.w == nil {
		return nil
	}
	if err := w.w.Flush(); err != nil {
		return err
	}
	return w.close()
}
