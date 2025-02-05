package artifacts

import (
	"io"
	"sync"

	"github.com/ethereum/go-ethereum/log"
	"github.com/schollz/progressbar/v3"
)

type DownloadProgressor func(current, total int64)

func BarProgressor() DownloadProgressor {
	var bar *progressbar.ProgressBar
	var init sync.Once
	return func(curr, total int64) {
		init.Do(func() {
			bar = progressbar.DefaultBytes(total)
		})
		_ = bar.Set64(curr)
	}
}

func NoopProgressor() DownloadProgressor {
	return func(curr, total int64) {}
}

func LogProgressor(lgr log.Logger) DownloadProgressor {
	return func(curr, total int64) {
		lgr.Info("artifacts download progress", "current", curr, "total", total)
	}
}

type progressReader struct {
	r        io.Reader
	progress DownloadProgressor
	curr     int64
	total    int64
}

func (pr *progressReader) Read(p []byte) (int, error) {
	n, err := pr.r.Read(p)
	pr.curr += int64(n)
	if pr.progress != nil {
		pr.progress(pr.curr, pr.total)
	}
	return n, err
}
