package testutil

import (
	"encoding/binary"
	"math/rand"

	"github.com/ethereum-optimism/optimism/cannon/mipsevm/arch"
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"
)

type RandHelper struct {
	r *rand.Rand
}

func NewRandHelper(seed int64) *RandHelper {
	r := rand.New(rand.NewSource(seed))
	return &RandHelper{r: r}
}

func (h *RandHelper) Uint32() uint32 {
	return h.r.Uint32()
}

func (h *RandHelper) Word() arch.Word {
	if arch.IsMips32 {
		return arch.Word(h.r.Uint32())
	} else {
		return arch.Word(h.r.Uint64())
	}
}

func (h *RandHelper) Fraction() float64 {
	return h.r.Float64()
}

func (h *RandHelper) Intn(n int) int {
	return h.r.Intn(n)
}

func (h *RandHelper) RandHash() common.Hash {
	var bytes [32]byte
	_, err := h.r.Read(bytes[:])
	if err != nil {
		panic(err)
	}
	return bytes
}

func (h *RandHelper) RandHint() []byte {

	bytesCount := h.r.Intn(24)
	bytes := make([]byte, bytesCount)

	if bytesCount >= 8 {
		// Set up a reasonable length prefix
		nextHintLen := uint64(h.r.Intn(30))
		binary.BigEndian.PutUint64(bytes, nextHintLen)

		_, err := h.r.Read(bytes[8:])
		if err != nil {
			panic(err)
		}
	}

	return bytes
}

func (h *RandHelper) RandRegisters() *[32]arch.Word {
	registers := new([32]arch.Word)
	for i := 0; i < 32; i++ {
		registers[i] = h.Word()
	}
	return registers
}

func (h *RandHelper) RandomBytes(t require.TestingT, length int) []byte {
	randBytes := make([]byte, length)
	if _, err := h.r.Read(randBytes); err != nil {
		require.NoError(t, err)
	}
	return randBytes
}

func (h *RandHelper) RandPC() arch.Word {
	return AlignPC(h.Word())
}

func (h *RandHelper) RandStep() uint64 {
	return BoundStep(h.r.Uint64())
}
