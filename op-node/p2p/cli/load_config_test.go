package cli

import (
	"errors"
	"net"
	"testing"

	"github.com/ethereum-optimism/optimism/op-node/p2p"
	"github.com/ethereum/go-ethereum/p2p/enode"
	"github.com/stretchr/testify/require"
)

func lookupIP(name string) ([]net.IP, error) {
	if name == "bootnode.conduit.xyz" {
		return []net.IP{{35, 197, 61, 230}}, nil
	}
	return nil, errors.New("no such host")
}

func TestResolveURLIP(t *testing.T) {
	for _, test := range []struct {
		url    string
		expUrl string
		expErr string
	}{
		{
			"enode://d25ce99435982b04d60c4b41ba256b84b888626db7bee45a9419382300fbe907359ae5ef250346785bff8d3b9d07cd3e017a27e2ee3cfda3bcbb0ba762ac9674@bootnode.conduit.xyz:0?discport=30301",
			"enode://d25ce99435982b04d60c4b41ba256b84b888626db7bee45a9419382300fbe907359ae5ef250346785bff8d3b9d07cd3e017a27e2ee3cfda3bcbb0ba762ac9674@35.197.61.230:0?discport=30301",
			"",
		},
		{
			"enode://869d07b5932f17e8490990f75a3f94195e9504ddb6b85f7189e5a9c0a8fff8b00aecf6f3ac450ecba6cdabdb5858788a94bde2b613e0f2d82e9b395355f76d1a@34.65.67.101:30305",
			"enode://869d07b5932f17e8490990f75a3f94195e9504ddb6b85f7189e5a9c0a8fff8b00aecf6f3ac450ecba6cdabdb5858788a94bde2b613e0f2d82e9b395355f76d1a@34.65.67.101:30305",
			"",
		},
		{
			"enode://2d4e7e9d48f4dd4efe9342706dd1b0024681bd4c3300d021f86fc75eab7865d4e0cbec6fbc883f011cfd6a57423e7e2f6e104baad2b744c3cafaec6bc7dc92c1@34.65.43.171:0?discport=30305",
			"enode://2d4e7e9d48f4dd4efe9342706dd1b0024681bd4c3300d021f86fc75eab7865d4e0cbec6fbc883f011cfd6a57423e7e2f6e104baad2b744c3cafaec6bc7dc92c1@34.65.43.171:0?discport=30305",
			"",
		},
		{
			"enode://d25ce99435982b04d60c4b41ba256b84b888626db7bee45a9419382300fbe907359ae5ef250346785bff8d3b9d07cd3e017a27e2ee3cfda3bcbb0ba762ac9674@bootnode.foo.bar:0?discport=30301",
			"",
			"no such host",
		},
		{
			"enode://d25ce99435982b04d60c4b41ba256b84b888626db7bee45a9419382300fbe907359ae5ef250346785bff8d3b9d07cd3e017a27e2ee3cfda3bcbb0ba762ac9674@::ffff:35.197.61.230:0?discport=30301",
			"enode://d25ce99435982b04d60c4b41ba256b84b888626db7bee45a9419382300fbe907359ae5ef250346785bff8d3b9d07cd3e017a27e2ee3cfda3bcbb0ba762ac9674@35.197.61.230:0?discport=30301",
			"",
		},
	} {
		u, err := resolveURLIP(test.url, lookupIP)
		if test.expErr != "" {
			require.Contains(t, err.Error(), test.expErr)
		} else {
			require.NoError(t, err)
			require.Equal(t, test.expUrl, u)
		}
	}
}

// TestDefaultBootnodes checks that the default bootnodes are valid enode specifiers.
// The default boodnodes use to be specified with [enode.MustParse]. But then upstream geth
// stopped resolving DNS host names in old enode specifiers. So this resolution got moved
// into the op-node's initP2P function. Because it is only run at runtime, this test
// ensures that the specifiers are valid (without DNS resolution, which is fine).
func TestDefaultBootnodes(t *testing.T) {
	for _, record := range p2p.DefaultBootnodes {
		nodeRecord, err := enode.Parse(enode.ValidSchemes, record)
		require.NoError(t, err)
		require.NotNil(t, nodeRecord)
	}
}
