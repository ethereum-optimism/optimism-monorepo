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

func NewLocatorFromTag(tag string) (*Locator, error) {
	loc := new(Locator)
	if err := loc.UnmarshalText([]byte("tag://" + tag)); err != nil {
		return nil, fmt.Errorf("failed to unmarshal tag: %w", err)
	}
	return loc, nil
}

func MustNewLocatorFromTag(tag string) *Locator {
	loc, err := NewLocatorFromTag(tag)
	if err != nil {
		panic(err)
	}
	return loc
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

	return []byte("tag://" + a.Tag), nil
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
