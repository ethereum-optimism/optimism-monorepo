package solabi

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math/big"

	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum/go-ethereum/common"
)

// These are empty padding values. They should be zero'd & not modified at all.
var (
	addressEmptyPadding [12]byte = [12]byte{}
	uint64EmptyPadding  [24]byte = [24]byte{}
	uint8EmptyPadding   [31]byte = [31]byte{}
)

func ReadSignature(r io.Reader) ([]byte, error) {
	sig := make([]byte, 4)
	_, err := io.ReadFull(r, sig)
	return sig, err
}

func ReadAndValidateSignature(r io.Reader, expectedSignature []byte) ([]byte, error) {
	sig := make([]byte, 4)
	if _, err := io.ReadFull(r, sig); err != nil {
		return nil, err
	}
	if !bytes.Equal(sig, expectedSignature) {
		return nil, errors.New("invalid function signature")
	}
	return sig, nil
}

func ReadHash(r io.Reader) (common.Hash, error) {
	var h common.Hash
	_, err := io.ReadFull(r, h[:])
	return h, err
}

func ReadEthBytes32(r io.Reader) (eth.Bytes32, error) {
	var b eth.Bytes32
	_, err := io.ReadFull(r, b[:])
	return b, err
}

func ReadAddress(r io.Reader) (common.Address, error) {
	var readPadding [12]byte
	var a common.Address
	if _, err := io.ReadFull(r, readPadding[:]); err != nil {
		return a, err
	} else if !bytes.Equal(readPadding[:], addressEmptyPadding[:]) {
		return a, fmt.Errorf("address padding was not empty: %x", readPadding[:])
	}
	_, err := io.ReadFull(r, a[:])
	return a, err
}

// ReadUint64 reads a big endian uint64 from a 32 byte word
func ReadUint64(r io.Reader) (uint64, error) {
	var readPadding [24]byte
	var n uint64
	if _, err := io.ReadFull(r, readPadding[:]); err != nil {
		return n, err
	} else if !bytes.Equal(readPadding[:], uint64EmptyPadding[:]) {
		return n, fmt.Errorf("number padding was not empty: %x", readPadding[:])
	}
	if err := binary.Read(r, binary.BigEndian, &n); err != nil {
		return 0, errors.New("expected number length to be 8 bytes")
	}
	return n, nil
}

func ReadUint256(r io.Reader) (*big.Int, error) {
	var n [32]byte
	if _, err := io.ReadFull(r, n[:]); err != nil {
		return nil, err
	}
	return new(big.Int).SetBytes(n[:]), nil
}

func EmptyReader(r io.Reader) bool {
	var t [1]byte
	n, err := r.Read(t[:])
	return n == 0 && err == io.EOF
}

func WriteSignature(w io.Writer, sig []byte) error {
	_, err := w.Write(sig)
	return err
}

func WriteHash(w io.Writer, h common.Hash) error {
	_, err := w.Write(h[:])
	return err
}

func WriteEthBytes32(w io.Writer, b eth.Bytes32) error {
	_, err := w.Write(b[:])
	return err
}

func WriteAddress(w io.Writer, a common.Address) error {
	if _, err := w.Write(addressEmptyPadding[:]); err != nil {
		return err
	}
	if _, err := w.Write(a[:]); err != nil {
		return err
	}
	return nil
}

func WriteUint256(w io.Writer, n *big.Int) error {
	if n.BitLen() > 256 {
		return fmt.Errorf("big int exceeds 256 bits: %d", n)
	}
	arr := make([]byte, 32)
	n.FillBytes(arr)
	_, err := w.Write(arr)
	return err
}

func WriteUint64(w io.Writer, n uint64) error {
	if _, err := w.Write(uint64EmptyPadding[:]); err != nil {
		return err
	}
	if err := binary.Write(w, binary.BigEndian, n); err != nil {
		return err
	}
	return nil
}

func WriteUint8(w io.Writer, n uint8) error {
	if _, err := w.Write(uint8EmptyPadding[:]); err != nil {
		return err
	}
	if err := binary.Write(w, binary.BigEndian, n); err != nil {
		return err
	}
	return nil
}
