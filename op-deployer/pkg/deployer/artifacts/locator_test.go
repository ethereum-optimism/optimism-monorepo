package artifacts

import (
	"net/url"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLocator_Marshaling(t *testing.T) {
	tests := []struct {
		name string
		in   string
		out  *Locator
		err  bool
	}{
		{
			name: "valid tag",
			in:   "tag://op-contracts/v1.6.0",
			out: &Locator{
				Tag: "op-contracts/v1.6.0",
			},
			err: false,
		},
		{
			name: "well-formed but nonexistent tag",
			in:   "tag://op-contracts/v1.5.0",
			out:  nil,
			err:  true,
		},
		{
			name: "mal-formed tag",
			in:   "tag://honk",
			out:  nil,
			err:  true,
		},
		{
			name: "valid HTTPS URL",
			in:   "https://example.com",
			out: &Locator{
				URL: parseUrl(t, "https://example.com"),
			},
			err: false,
		},
		{
			name: "valid HTTP URL",
			in:   "http://example.com",
			out: &Locator{
				URL: parseUrl(t, "http://example.com"),
			},
			err: false,
		},
		{
			name: "valid file URL",
			in:   "file:///tmp/artifacts",
			out: &Locator{
				URL: parseUrl(t, "file:///tmp/artifacts"),
			},
			err: false,
		},
		{
			name: "empty",
			in:   "",
			out:  nil,
			err:  true,
		},
		{
			name: "no scheme",
			in:   "example.com",
			out:  nil,
			err:  true,
		},
		{
			name: "unsupported scheme",
			in:   "ftp://example.com",
			out:  nil,
			err:  true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var a Locator
			err := a.UnmarshalText([]byte(tt.in))
			if tt.err {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.out, &a)

			marshalled, err := a.MarshalText()
			require.NoError(t, err)
			require.Equal(t, tt.in, string(marshalled))
		})
	}
}

func parseUrl(t *testing.T, u string) *url.URL {
	parsed, err := url.Parse(u)
	require.NoError(t, err)
	return parsed
}

func TestLocator_Equal(t *testing.T) {
	tests := []struct {
		a     *Locator
		b     *Locator
		equal bool
	}{
		{
			MustNewLocatorFromTag("op-contracts/v1.6.0"),
			MustNewLocatorFromTag("op-contracts/v1.8.0-rc.4"),
			false,
		},
		{
			MustNewLocatorFromTag("op-contracts/v1.6.0"),
			MustNewLocatorFromTag("op-contracts/v1.6.0"),
			true,
		},
		{
			MustNewLocatorFromURL("http://www.example.com"),
			MustNewLocatorFromTag("op-contracts/v1.6.0"),
			false,
		},
		{
			MustNewLocatorFromURL("https://www.example.com"),
			MustNewLocatorFromURL("http://www.example.com"),
			false,
		},
		{
			MustNewLocatorFromURL("http://www.example.com"),
			MustNewLocatorFromURL("http://www.example.com"),
			true,
		},
		{
			MustNewLocatorFromTag("op-contracts/v1.6.0"),
			MustNewFileLocator("/foo/bar"),
			false,
		},
		{
			MustNewFileLocator("/foo/bar"),
			MustNewFileLocator("/foo/bar"),
			true,
		},
		{
			MustNewFileLocator("/foo/bar"),
			MustNewFileLocator("/foo/baz"),
			false,
		},
	}
	for _, test := range tests {
		if test.equal {
			require.True(t, test.a.Equal(test.b), "%s != %s", test.a, test.b)
		} else {
			require.False(t, test.a.Equal(test.b), "%s == %s", test.a, test.b)
		}
	}
}
