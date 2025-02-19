package env

import (
	"fmt"
	"net/url"
	"os"
	"strings"
)

// fetchFileData reads data from a local file
func fetchFileData(u *url.URL) (string, []byte, error) {
	body, err := os.ReadFile(u.Path)
	if err != nil {
		return "", nil, fmt.Errorf("error reading file: %w", err)
	}

	basename := u.Path
	if lastSlash := strings.LastIndex(basename, "/"); lastSlash >= 0 {
		basename = basename[lastSlash+1:]
	}
	if lastDot := strings.LastIndex(basename, "."); lastDot >= 0 {
		basename = basename[:lastDot]
	}
	return basename, body, nil
}
