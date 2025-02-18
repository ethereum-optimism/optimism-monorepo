package mipsevm

import (
	"github.com/ethereum/go-ethereum/log"
)

// LoggingWriter is a simple util to wrap a logger,
// and expose an io Writer interface,
// for the program running within the VM to write to.
type LoggingWriter struct {
	Log log.Logger
}

func logAsText(b string) bool {
	for _, c := range b {
		if (c < 0x20 || c >= 0x7F) && (c != '\n' && c != '\t') {
			return false
		}
	}
	return true
}

func (lw *LoggingWriter) Write(b []byte) (int, error) {
	lw.Log.Info("", "text", string(b))
	return len(b), nil
}
