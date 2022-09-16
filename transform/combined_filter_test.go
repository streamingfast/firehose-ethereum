package transform

import (
	"strings"
	"testing"

	"github.com/streamingfast/eth-go"
	pbtransform "github.com/streamingfast/firehose-ethereum/types/pb/sf/ethereum/transform/v1"
	"github.com/stretchr/testify/assert"
	"github.com/test-go/testify/require"
)

func TestString(t *testing.T) {
	c, err := newCombinedFilter(nil, nil, nil, nil, false)
	require.NoError(t, err)
	assert.Equal(t, "Combined filter: Calls:[], Logs:[], SendAllBlockHeaders: false", c.String())

	cf := &pbtransform.CallToFilter{
		Addresses:  [][]byte{eth.MustNewHex("0xdeadbeef")},
		Signatures: [][]byte{eth.MustNewHex("0xbbbb")},
	}
	cf2 := &pbtransform.CallToFilter{
		Addresses:  [][]byte{eth.MustNewHex("0x" + strings.Repeat("9", 100))},
		Signatures: [][]byte{eth.MustNewHex("0xbbbb")},
	}
	lf1 := &pbtransform.LogFilter{
		Addresses:       [][]byte{eth.MustNewHex("0xdeadbeef")},
		EventSignatures: [][]byte{eth.MustNewHex("0xbbbb")},
	}
	lf2 := &pbtransform.LogFilter{
		Addresses:       [][]byte{eth.MustNewHex("0xcccc2222")},
		EventSignatures: nil,
	}
	lf3 := &pbtransform.LogFilter{
		Addresses:       [][]byte{eth.MustNewHex("0x" + strings.Repeat("9", 100))},
		EventSignatures: nil,
	}

	c, err = newCombinedFilter([]*pbtransform.CallToFilter{cf, cf2}, []*pbtransform.LogFilter{lf1, lf2, lf3}, nil, nil, true)
	require.NoError(t, err)
	assert.Equal(t, "Combined filter: Calls:[{addrs: 0xdeadbeef, sigs: 0xbbbb},{addrs: 0x9999999999999999999999999999999999999999999999...}], Logs:[{addrs: 0xdeadbeef, sigs: 0xbbbb},{addrs: 0xcccc2222, sigs: },{addrs: 0x999999999999999999...}], SendAllBlockHeaders: true", c.String())
}
