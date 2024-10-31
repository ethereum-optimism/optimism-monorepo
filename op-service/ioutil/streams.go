package ioutil

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
)

var (
	stdOutStream OutputTarget = func() (io.Writer, io.Closer, Aborter, error) {
		return os.Stdout, &noopCloser{}, func() {}, nil
	}
)

type Aborter func()

type OutputTarget func() (io.Writer, io.Closer, Aborter, error)

func NoOutputStream() OutputTarget {
	return func() (io.Writer, io.Closer, Aborter, error) {
		return nil, nil, nil, nil
	}
}

func ToBasicFile(path string, perm os.FileMode) OutputTarget {
	return func() (io.Writer, io.Closer, Aborter, error) {
		outDir := filepath.Dir(path)
		if err := os.MkdirAll(outDir, perm); err != nil {
			return nil, nil, nil, fmt.Errorf("failed to create dir %q: %w", outDir, err)
		}
		f, err := os.OpenFile(path, os.O_WRONLY|os.O_TRUNC|os.O_CREATE, perm)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("failed to open %q: %w", path, err)
		}
		return f, f, func() {}, nil
	}
}

func ToAtomicFile(path string, perm os.FileMode) OutputTarget {
	return func() (io.Writer, io.Closer, Aborter, error) {
		f, err := NewAtomicWriterCompressed(path, perm)
		if err != nil {
			return nil, nil, nil, err
		}
		return f, f, func() { _ = f.Abort() }, nil
	}
}

func ToStdOut() OutputTarget {
	return stdOutStream
}

func ToStdOutOrFileOrNoop(outputPath string, perm os.FileMode) OutputTarget {
	if outputPath == "" {
		return NoOutputStream()
	} else if outputPath == "-" {
		return ToStdOut()
	} else {
		return ToAtomicFile(outputPath, perm)
	}
}

type noopCloser struct{}

func (c *noopCloser) Close() error {
	return nil
}
