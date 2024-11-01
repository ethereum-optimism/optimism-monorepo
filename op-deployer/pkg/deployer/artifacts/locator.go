package artifacts

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/ethereum-optimism/optimism/op-deployer/pkg/deployer/standard"
)

type schemeUnmarshaler func(string) (*Locator, error)

var schemeUnmarshalerDispatch = map[string]schemeUnmarshaler{
	"tag":   unmarshalTag,
	"file":  unmarshalURL,
	"https": unmarshalURL,
}

var DefaultL1ContractsLocator = &Locator{
	Tag: standard.DefaultL1ContractsTag,
}

var DefaultL2ContractsLocator = &Locator{
	Tag: standard.DefaultL2ContractsTag,
}

type Locator struct {
	URL *url.URL
	Tag string
}

func (a *Locator) UnmarshalText(text []byte) error {
	str := string(text)

	for scheme, unmarshaler := range schemeUnmarshalerDispatch {
		if !strings.HasPrefix(str, scheme+"://") {
			continue
		}

		loc, err := unmarshaler(str)
		if err != nil {
			return err
		}

		*a = *loc
		return nil
	}

	return fmt.Errorf("unsupported scheme")
}

func (a *Locator) MarshalText() ([]byte, error) {
	if a.URL != nil {
		return []byte(a.URL.String()), nil
	}

	if a.Tag != "" {
		return []byte("tag://" + a.Tag), nil
	}

	return nil, fmt.Errorf("no URL, path or tag set")
}

func (a *Locator) IsTag() bool {
	return a.Tag != ""
}

func unmarshalTag(tag string) (*Locator, error) {
	tag = strings.TrimPrefix(tag, "tag://")
	if !strings.HasPrefix(tag, "op-contracts/") {
		return nil, fmt.Errorf("invalid tag: %s", tag)
	}

	if _, err := standard.ArtifactsURLForTag(tag); err != nil {
		return nil, err
	}

	return &Locator{Tag: tag}, nil
}

func unmarshalURL(text string) (*Locator, error) {
	u, err := url.Parse(text)
	if err != nil {
		return nil, err
	}

	return &Locator{URL: u}, nil
}
