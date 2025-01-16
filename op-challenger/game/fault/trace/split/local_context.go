package split

import (
	"github.com/ethereum-optimism/optimism/op-challenger/game/fault/types"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
)

func CreateLocalContext(pre types.Claim, post types.Claim) common.Hash {
	return crypto.Keccak256Hash(LocalContextPreimage(pre, post))
}

func LocalContextPreimage(pre types.Claim, post types.Claim) []byte {
	encodeClaim := func(c types.Claim) []byte {
		data := make([]byte, 64)
		copy(data[0:32], c.Value.Bytes())
		c.Position.ToGIndex().FillBytes(data[32:])
		return data
	}
	var data []byte
	if pre != (types.Claim{}) {
		data = encodeClaim(pre)
	}
	data = append(data, encodeClaim(post)...)
	return data
}
