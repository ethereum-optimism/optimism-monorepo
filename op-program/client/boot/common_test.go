package boot

import (
	"encoding/binary"

	preimage "github.com/ethereum-optimism/optimism/op-preimage"
	"github.com/ethereum/go-ethereum/common"
)

type mockBoostrapOracle struct {
	l1Head             common.Hash
	l2OutputRoot       common.Hash
	l2Claim            common.Hash
	l2ClaimBlockNumber uint64
}

func (o *mockBoostrapOracle) Get(key preimage.Key) []byte {
	switch key.PreimageKey() {
	case L1HeadLocalIndex.PreimageKey():
		return o.l1Head[:]
	case L2OutputRootLocalIndex.PreimageKey():
		return o.l2OutputRoot[:]
	case L2ClaimLocalIndex.PreimageKey():
		return o.l2Claim[:]
	case L2ClaimBlockNumberLocalIndex.PreimageKey():
		return binary.BigEndian.AppendUint64(nil, o.l2ClaimBlockNumber)
	default:
		panic("unknown key")
	}
}
