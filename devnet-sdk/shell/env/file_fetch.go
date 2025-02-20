package env

import (
	"fmt"
	"net/url"
	"os"
	"strings"
)

type osInterface interface {
	ReadFile(name string) ([]byte, error)
}

type defaultOS struct{}

func (d *defaultOS) ReadFile(name string) ([]byte, error) {
	return os.ReadFile(name)
}

// fetchFileData reads data from a local file
func fetchFileDataFromOS(u *url.URL, os osInterface) (string, []byte, error) {
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

func fetchFileData(u *url.URL) (string, []byte, error) {
	return fetchFileDataFromOS(u, &defaultOS{})
}
