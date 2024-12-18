package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"math"
	"math/big"
	"os"

	"github.com/ethereum-optimism/optimism/op-service/eth"
)

// These names match those used in the SystemConfig contract
type outputTy struct {
	BaseFee           uint     `json:"baseFeeScalar"`
	BlobbaseFeeScalar uint     `json:"blobbaseFeeScalar"`
	ScalarHex         string   `json:"scalarHex"`
	Scalar            *big.Int `json:"scalar"` // post-ecotone
}

func main() {
	var scalar, blobScalar uint
	var decode string
	flag.StringVar(&decode, "decode", "", "uint256 post-ecotone scalar value to decode into its components")
	flag.UintVar(&scalar, "scalar", 0, "base fee scalar value for the gas config (uint32)")
	flag.UintVar(&blobScalar, "blob-scalar", 0, "blob base fee scalar value for the gas config (uint32)")
	flag.Parse()

	if scalar > math.MaxUint32 {
		fmt.Fprintln(flag.CommandLine.Output(), "scalar out of uint32 range")
		flag.Usage()
		os.Exit(2)
	}
	if blobScalar > math.MaxUint32 {
		fmt.Fprintln(flag.CommandLine.Output(), "blob-scalar out of uint32 range")
		flag.Usage()
		os.Exit(2)
	}

	var encoded [32]byte
	if len(decode) > 0 {
		if scalar != 0 || blobScalar != 0 {
			fmt.Fprintln(flag.CommandLine.Output(), "decode parameter should not be used with scalar and blob-scalar")
			flag.Usage()
			os.Exit(2)
		}
		uint256 := new(big.Int)
		_, ok := uint256.SetString(decode, 0)
		if !ok {
			fmt.Fprintln(flag.CommandLine.Output(), "failed to parse int from post-ecotone scalar")
			flag.Usage()
			os.Exit(2)
		}
		byteLen := (uint256.BitLen() + 7) / 8
		if byteLen > 32 {
			fmt.Fprintln(flag.CommandLine.Output(), "post-ecotone scalar out of uint256 range")
			flag.Usage()
			os.Exit(2)
		}
		uint256.FillBytes(encoded[:])
		decoded, err := eth.DecodeScalar(encoded)
		if err != nil {
			fmt.Fprintln(flag.CommandLine.Output(), "post-ecotone scalar could not be decoded:", err)
			flag.Usage()
			os.Exit(2)
		}
		scalar = uint(decoded.BaseFeeScalar)
		blobScalar = uint(decoded.BlobBaseFeeScalar)
	} else {
		encoded = eth.EncodeScalar(eth.EcotoneScalars{
			BlobBaseFeeScalar: uint32(blobScalar),
			BaseFeeScalar:     uint32(scalar),
		})
	}
	i := new(big.Int).SetBytes(encoded[:])

	o, err := json.Marshal(outputTy{
		BaseFee:           scalar,
		BlobbaseFeeScalar: blobScalar,
		ScalarHex:         fmt.Sprintf("0x%x", encoded[:]),
		Scalar:            i,
	})
	if err != nil {
		panic(err)
	}
	fmt.Println(string(o))
}
