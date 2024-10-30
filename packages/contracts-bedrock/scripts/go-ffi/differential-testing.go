package main

import (
	"bytes"
	"fmt"
	"math/big"
	"os"
	"strconv"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/rawdb"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/trie"
	"github.com/ethereum/go-ethereum/triedb"
	"github.com/ethereum/go-ethereum/triedb/hashdb"

	"github.com/ethereum-optimism/optimism/cannon/mipsevm/arch"
	"github.com/ethereum-optimism/optimism/cannon/mipsevm/memory"
	"github.com/ethereum-optimism/optimism/op-chain-ops/crossdomain"
	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum-optimism/optimism/op-service/predeploys"
)

// ABI types
var (
	// Plain dynamic dynBytes type
	dynBytes, _ = abi.NewType("bytes", "", nil)
	bytesArgs   = abi.Arguments{
		{Type: dynBytes},
	}

	// Plain fixed bytes32 type
	fixedBytes, _  = abi.NewType("bytes32", "", nil)
	fixedBytesArgs = abi.Arguments{
		{Type: fixedBytes},
	}

	uint32Type, _ = abi.NewType("uint32", "", nil)

	// Plain address type
	addressType, _ = abi.NewType("address", "", nil)

	// Plain uint8 type
	uint8Type, _ = abi.NewType("uint8", "", nil)

	// Plain uint256 type
	uint256Type, _ = abi.NewType("uint256", "", nil)

	// Decoded nonce tuple (nonce, version)
	decodedNonce, _ = abi.NewType("tuple", "DecodedNonce", []abi.ArgumentMarshaling{
		{Name: "nonce", Type: "uint256"},
		{Name: "version", Type: "uint256"},
	})
	decodedNonceArgs = abi.Arguments{
		{Name: "encodedNonce", Type: decodedNonce},
	}

	// Decoded ecotone scalars (uint32, uint32)
	decodedScalars = abi.Arguments{
		{Name: "basefeeScalar", Type: uint32Type},
		{Name: "blobbasefeeScalar", Type: uint32Type},
	}

	// WithdrawalHash slot tuple (bytes32, bytes32)
	withdrawalSlot, _ = abi.NewType("tuple", "SlotHash", []abi.ArgumentMarshaling{
		{Name: "withdrawalHash", Type: "bytes32"},
		{Name: "zeroPadding", Type: "bytes32"},
	})
	withdrawalSlotArgs = abi.Arguments{
		{Name: "slotHash", Type: withdrawalSlot},
	}

	// Prove withdrawal inputs tuple (bytes32, bytes32, bytes32, bytes32, bytes[])
	proveWithdrawalInputs, _ = abi.NewType("tuple", "ProveWithdrawalInputs", []abi.ArgumentMarshaling{
		{Name: "worldRoot", Type: "bytes32"},
		{Name: "stateRoot", Type: "bytes32"},
		{Name: "outputRoot", Type: "bytes32"},
		{Name: "withdrawalHash", Type: "bytes32"},
		{Name: "proof", Type: "bytes[]"},
	})
	proveWithdrawalInputsArgs = abi.Arguments{
		{Name: "inputs", Type: proveWithdrawalInputs},
	}

	// cannonMemoryProof inputs tuple (bytes32, bytes)
	cannonMemoryProof, _ = abi.NewType("tuple", "CannonMemoryProof", []abi.ArgumentMarshaling{
		{Name: "memRoot", Type: "bytes32"},
		{Name: "proof", Type: "bytes"},
	})
	cannonMemoryProofArgs = abi.Arguments{
		{Name: "encodedCannonMemoryProof", Type: cannonMemoryProof},
	}

	// Gas paying token tuple (address, uint8, bytes32, bytes32)
	gasPayingTokenArgs = abi.Arguments{
		{Name: "token", Type: addressType},
		{Name: "decimals", Type: uint8Type},
		{Name: "name", Type: fixedBytes},
		{Name: "symbol", Type: fixedBytes},
	}

	// Dependency tuple (uint256)
	dependencyArgs = abi.Arguments{{Name: "chainId", Type: uint256Type}}
)

func DiffTestUtils() {
	args := os.Args[2:]
	variant := args[0]

	// This command requires arguments
	if len(args) == 0 {
		panic("Error: No arguments provided")
	}

	switch variant {
	case "decodeVersionedNonce":
		// Parse input arguments
		input, ok := new(big.Int).SetString(args[1], 10)
		checkOk(ok)

		// Decode versioned nonce
		nonce, version := crossdomain.DecodeVersionedNonce(input)

		// ABI encode output
		packArgs := struct {
			Nonce   *big.Int
			Version *big.Int
		}{
			nonce,
			version,
		}
		packed, err := decodedNonceArgs.Pack(&packArgs)
		checkErr(err, "Error encoding output")

		fmt.Print(hexutil.Encode(packed))
	case "encodeCrossDomainMessage":
		// Parse input arguments
		nonce, ok := new(big.Int).SetString(args[1], 10)
		checkOk(ok)
		sender := common.HexToAddress(args[2])
		target := common.HexToAddress(args[3])
		value, ok := new(big.Int).SetString(args[4], 10)
		checkOk(ok)
		gasLimit, ok := new(big.Int).SetString(args[5], 10)
		checkOk(ok)
		data := common.FromHex(args[6])

		// Encode cross domain message
		encoded, err := encodeCrossDomainMessage(nonce, sender, target, value, gasLimit, data)
		checkErr(err, "Error encoding cross domain message")

		// Pack encoded cross domain message
		packed, err := bytesArgs.Pack(&encoded)
		checkErr(err, "Error encoding output")

		fmt.Print(hexutil.Encode(packed))
	case "hashCrossDomainMessage":
		// Parse input arguments
		nonce, ok := new(big.Int).SetString(args[1], 10)
		checkOk(ok)
		sender := common.HexToAddress(args[2])
		target := common.HexToAddress(args[3])
		value, ok := new(big.Int).SetString(args[4], 10)
		checkOk(ok)
		gasLimit, ok := new(big.Int).SetString(args[5], 10)
		checkOk(ok)
		data := common.FromHex(args[6])

		// Encode cross domain message
		encoded, err := encodeCrossDomainMessage(nonce, sender, target, value, gasLimit, data)
		checkErr(err, "Error encoding cross domain message")

		// Hash encoded cross domain message
		hash := crypto.Keccak256Hash(encoded)

		// Pack hash
		packed, err := fixedBytesArgs.Pack(&hash)
		checkErr(err, "Error encoding output")

		fmt.Print(hexutil.Encode(packed))
	case "hashDepositTransaction":
		// Parse input arguments
		l1BlockHash := common.HexToHash(args[1])
		logIndex, ok := new(big.Int).SetString(args[2], 10)
		checkOk(ok)
		from := common.HexToAddress(args[3])
		to := common.HexToAddress(args[4])
		mint, ok := new(big.Int).SetString(args[5], 10)
		checkOk(ok)
		value, ok := new(big.Int).SetString(args[6], 10)
		checkOk(ok)
		gasLimit, ok := new(big.Int).SetString(args[7], 10)
		checkOk(ok)
		data := common.FromHex(args[8])

		// Create deposit transaction
		depositTx := makeDepositTx(from, to, value, mint, gasLimit, false, data, l1BlockHash, logIndex)

		// RLP encode deposit transaction
		encoded, err := types.NewTx(&depositTx).MarshalBinary()
		checkErr(err, "Error encoding deposit transaction")

		// Hash encoded deposit transaction
		hash := crypto.Keccak256Hash(encoded)

		// Pack hash
		packed, err := fixedBytesArgs.Pack(&hash)
		checkErr(err, "Error encoding output")

		fmt.Print(hexutil.Encode(packed))
	case "encodeDepositTransaction":
		// Parse input arguments
		from := common.HexToAddress(args[1])
		to := common.HexToAddress(args[2])
		value, ok := new(big.Int).SetString(args[3], 10)
		checkOk(ok)
		mint, ok := new(big.Int).SetString(args[4], 10)
		checkOk(ok)
		gasLimit, ok := new(big.Int).SetString(args[5], 10)
		checkOk(ok)
		isCreate := args[6] == "true"
		data := common.FromHex(args[7])
		l1BlockHash := common.HexToHash(args[8])
		logIndex, ok := new(big.Int).SetString(args[9], 10)
		checkOk(ok)

		depositTx := makeDepositTx(from, to, value, mint, gasLimit, isCreate, data, l1BlockHash, logIndex)

		// RLP encode deposit transaction
		encoded, err := types.NewTx(&depositTx).MarshalBinary()
		checkErr(err, "Failed to RLP encode deposit transaction")
		// Pack rlp encoded deposit transaction
		packed, err := bytesArgs.Pack(&encoded)
		checkErr(err, "Error encoding output")

		fmt.Print(hexutil.Encode(packed))
	case "hashWithdrawal":
		// Parse input arguments
		nonce, ok := new(big.Int).SetString(args[1], 10)
		checkOk(ok)
		sender := common.HexToAddress(args[2])
		target := common.HexToAddress(args[3])
		value, ok := new(big.Int).SetString(args[4], 10)
		checkOk(ok)
		gasLimit, ok := new(big.Int).SetString(args[5], 10)
		checkOk(ok)
		data := common.FromHex(args[6])

		// Hash withdrawal
		hash, err := hashWithdrawal(nonce, sender, target, value, gasLimit, data)
		checkErr(err, "Error hashing withdrawal")

		// Pack hash
		packed, err := fixedBytesArgs.Pack(&hash)
		checkErr(err, "Error encoding output")

		fmt.Print(hexutil.Encode(packed))
	case "hashOutputRootProof":
		// Parse input arguments
		version := common.HexToHash(args[1])
		stateRoot := common.HexToHash(args[2])
		messagePasserStorageRoot := common.HexToHash(args[3])
		latestBlockHash := common.HexToHash(args[4])

		// Hash the output root proof
		hash, err := hashOutputRootProof(version, stateRoot, messagePasserStorageRoot, latestBlockHash)
		checkErr(err, "Error hashing output root proof")

		// Pack hash
		packed, err := fixedBytesArgs.Pack(&hash)
		checkErr(err, "Error encoding output")

		fmt.Print(hexutil.Encode(packed))
	case "getProveWithdrawalTransactionInputs":
		// Parse input arguments
		nonce, ok := new(big.Int).SetString(args[1], 10)
		checkOk(ok)
		sender := common.HexToAddress(args[2])
		target := common.HexToAddress(args[3])
		value, ok := new(big.Int).SetString(args[4], 10)
		checkOk(ok)
		gasLimit, ok := new(big.Int).SetString(args[5], 10)
		checkOk(ok)
		data := common.FromHex(args[6])

		wdHash, err := hashWithdrawal(nonce, sender, target, value, gasLimit, data)
		checkErr(err, "Error hashing withdrawal")

		// Compute the storage slot the withdrawalHash will be stored in
		slot := struct {
			WithdrawalHash common.Hash
			ZeroPadding    common.Hash
		}{
			WithdrawalHash: wdHash,
			ZeroPadding:    common.Hash{},
		}
		packed, err := withdrawalSlotArgs.Pack(&slot)
		checkErr(err, "Error packing withdrawal slot")

		// Compute the storage slot the withdrawalHash will be stored in
		hash := crypto.Keccak256Hash(packed)

		// Create a secure trie for state
		state, err := trie.NewStateTrie(
			trie.TrieID(types.EmptyRootHash),
			triedb.NewDatabase(rawdb.NewMemoryDatabase(), &triedb.Config{HashDB: hashdb.Defaults}),
		)
		checkErr(err, "Error creating secure trie")

		// Put a "true" bool in the storage slot
		err = state.UpdateStorage(common.Address{}, hash.Bytes(), []byte{0x01})
		checkErr(err, "Error updating storage")

		// Create a secure trie for the world state
		world, err := trie.NewStateTrie(
			trie.TrieID(types.EmptyRootHash),
			triedb.NewDatabase(rawdb.NewMemoryDatabase(), &triedb.Config{HashDB: hashdb.Defaults}),
		)
		checkErr(err, "Error creating secure trie")

		// Put the put the rlp encoded account in the world trie
		account := types.StateAccount{
			Nonce:   0,
			Balance: common.U2560,
			Root:    state.Hash(),
		}
		writer := new(bytes.Buffer)
		checkErr(account.EncodeRLP(writer), "Error encoding account")
		err = world.UpdateStorage(common.Address{}, predeploys.L2ToL1MessagePasserAddr.Bytes(), writer.Bytes())
		checkErr(err, "Error updating storage")

		// Get the proof
		var proof proofList
		checkErr(state.Prove(predeploys.L2ToL1MessagePasserAddr.Bytes(), &proof), "Error getting proof")

		// Get the output root
		outputRoot, err := hashOutputRootProof(common.Hash{}, world.Hash(), state.Hash(), common.Hash{})
		checkErr(err, "Error hashing output root proof")

		// Pack the output
		output := struct {
			WorldRoot      common.Hash
			StateRoot      common.Hash
			OutputRoot     common.Hash
			WithdrawalHash common.Hash
			Proof          proofList
		}{
			WorldRoot:      world.Hash(),
			StateRoot:      state.Hash(),
			OutputRoot:     outputRoot,
			WithdrawalHash: wdHash,
			Proof:          proof,
		}
		packed, err = proveWithdrawalInputsArgs.Pack(&output)
		checkErr(err, "Error encoding output")

		// Print the output
		fmt.Print(hexutil.Encode(packed[32:]))
	case "cannonMemoryProof":
		// <memAddr0, memValue0, [memAddr1, memValue1], [memAddr2, memValue2]>
		// Generates memory proofs of `memAddr0` for a trie containing memValue0 and `memAddr1` for a trie containing memValue1 and memValue2
		// For the cannon stf, this is equivalent to the prestate proofs of the program counter and memory access for instruction execution
		mem := memory.NewMemory()
		if len(args) != 3 && len(args) != 5 && len(args) != 7 {
			panic("Error: cannonMemoryProofWithProof requires 2, 4, or 6 arguments")
		}
		memAddr0, err := strconv.ParseUint(args[1], 10, arch.WordSize)
		checkErr(err, "Error decoding addr")
		memValue0, err := strconv.ParseUint(args[2], 10, arch.WordSize)
		checkErr(err, "Error decoding memValue0")
		mem.SetWord(arch.Word(memAddr0), arch.Word(memValue0))

		var proof1 []byte
		if len(args) >= 5 {
			memAddr, err := strconv.ParseUint(args[3], 10, arch.WordSize)
			checkErr(err, "Error decoding memAddr")
			memValue, err := strconv.ParseUint(args[4], 10, arch.WordSize)
			checkErr(err, "Error decoding memValue")
			mem.SetWord(arch.Word(memAddr), arch.Word(memValue))
			proof := mem.MerkleProof(arch.Word(memAddr))
			proof1 = proof[:]
		}
		if len(args) == 7 {
			memAddr, err := strconv.ParseUint(args[5], 10, arch.WordSize)
			checkErr(err, "Error decoding memAddr")
			memValue, err := strconv.ParseUint(args[6], 10, arch.WordSize)
			checkErr(err, "Error decoding memValue")
			mem.SetWord(arch.Word(memAddr), arch.Word(memValue))
			proof := mem.MerkleProof(arch.Word(memAddr))
			proof1 = proof[:]
		}
		proof0 := mem.MerkleProof(arch.Word(memAddr0))

		output := struct {
			MemRoot common.Hash
			Proof   []byte
		}{
			MemRoot: mem.MerkleRoot(),
			Proof:   append(proof0[:], proof1...),
		}
		packed, err := cannonMemoryProofArgs.Pack(&output)
		checkErr(err, "Error encoding output")
		fmt.Print(hexutil.Encode(packed[32:]))
	case "cannonMemoryProof2":
		// <memAddr0, memValue0, [memAddr1, memValue1], memAddr2>
		// Generates memory proof of `memAddr2` for a trie containing `memValue0` and `memValue1`
		mem := memory.NewMemory()
		if len(args) != 6 {
			panic("Error: cannonMemoryProofWithProof2 requires 5 arguments")
		}
		memAddr0, err := strconv.ParseUint(args[1], 10, arch.WordSize)
		checkErr(err, "Error decoding addr")
		memValue0, err := strconv.ParseUint(args[2], 10, arch.WordSize)
		checkErr(err, "Error decoding memValue0")
		mem.SetWord(arch.Word(memAddr0), arch.Word(memValue0))

		var memProof [memory.MemProofSize]byte
		memAddr, err := strconv.ParseUint(args[3], 10, arch.WordSize)
		checkErr(err, "Error decoding memAddr")
		memValue1, err := strconv.ParseUint(args[4], 10, arch.WordSize)
		checkErr(err, "Error decoding memValue1")
		mem.SetWord(arch.Word(memAddr), arch.Word(memValue1))

		memAddr2, err := strconv.ParseUint(args[5], 10, arch.WordSize)
		checkErr(err, "Error decoding memAddr")
		memProof = mem.MerkleProof(arch.Word(memAddr2))

		output := struct {
			MemRoot common.Hash
			Proof   []byte
		}{
			MemRoot: mem.MerkleRoot(),
			Proof:   memProof[:],
		}
		packed, err := cannonMemoryProofArgs.Pack(&output)
		checkErr(err, "Error encoding output")
		fmt.Print(hexutil.Encode(packed[32:]))
	case "cannonMemoryProofWrongLeaf":
		// <memAddr0, memValue0, memAddr1, memValue1>
		mem := memory.NewMemory()
		if len(args) != 5 {
			panic("Error: cannonMemoryProofWrongLeaf requires 4 arguments")
		}
		memAddr0, err := strconv.ParseUint(args[1], 10, arch.WordSize)
		checkErr(err, "Error decoding memAddr0")
		memValue0, err := strconv.ParseUint(args[2], 10, arch.WordSize)
		checkErr(err, "Error decoding memValue0")
		mem.SetWord(arch.Word(memAddr0), arch.Word(memValue0))

		var insnProof, memProof [memory.MemProofSize]byte
		memAddr1, err := strconv.ParseUint(args[3], 10, arch.WordSize)
		checkErr(err, "Error decoding memAddr1")
		memValue1, err := strconv.ParseUint(args[4], 10, arch.WordSize)
		checkErr(err, "Error decoding memValue1")
		mem.SetWord(arch.Word(memAddr1), arch.Word(memValue1))

		// Compute a valid proof for the root, but for the wrong leaves.
		memProof = mem.MerkleProof(arch.Word(memAddr1 + arch.WordSize))
		insnProof = mem.MerkleProof(arch.Word(memAddr0 + arch.WordSize))

		output := struct {
			MemRoot common.Hash
			Proof   []byte
		}{
			MemRoot: mem.MerkleRoot(),
			Proof:   append(insnProof[:], memProof[:]...),
		}
		packed, err := cannonMemoryProofArgs.Pack(&output)
		checkErr(err, "Error encoding output")
		fmt.Print(hexutil.Encode(packed[32:]))
	case "encodeScalarEcotone":
		basefeeScalar, err := strconv.ParseUint(args[1], 10, 32)
		checkErr(err, "Error decoding basefeeScalar")
		blobbasefeeScalar, err := strconv.ParseUint(args[2], 10, 32)
		checkErr(err, "Error decoding blobbasefeeScalar")

		encoded := eth.EncodeScalar(eth.EcotoneScalars{
			BaseFeeScalar:     uint32(basefeeScalar),
			BlobBaseFeeScalar: uint32(blobbasefeeScalar),
		})
		fmt.Print(hexutil.Encode(encoded[:]))
	case "decodeScalarEcotone":
		scalar := common.HexToHash(args[1])
		scalars, err := eth.DecodeScalar([32]byte(scalar[:]))
		checkErr(err, "Error decoding scalar")

		packed, err := decodedScalars.Pack(scalars.BaseFeeScalar, scalars.BlobBaseFeeScalar)
		checkErr(err, "Error encoding output")
		fmt.Print(hexutil.Encode(packed))
	case "encodeGasPayingToken":
		// Parse input arguments
		token := common.HexToAddress(args[1])
		decimals, err := strconv.ParseUint(args[2], 10, 8)
		checkErr(err, "Error decoding decimals")
		name := common.HexToHash(args[3])
		symbol := common.HexToHash(args[4])

		// Encode gas paying token
		encoded, err := gasPayingTokenArgs.Pack(token, uint8(decimals), name, symbol)
		checkErr(err, "Error encoding gas paying token")

		// Pack encoded gas paying token
		packed, err := bytesArgs.Pack(&encoded)
		checkErr(err, "Error encoding output")

		fmt.Print(hexutil.Encode(packed))
	case "encodeDependency":
		// Parse input arguments
		chainId, ok := new(big.Int).SetString(args[1], 10)
		checkOk(ok)

		// Encode dependency
		encoded, err := dependencyArgs.Pack(chainId)
		checkErr(err, "Error encoding dependency")

		// Pack encoded dependency
		packed, err := bytesArgs.Pack(&encoded)
		checkErr(err, "Error encoding output")

		fmt.Print(hexutil.Encode(packed))
	default:
		panic(fmt.Errorf("Unknown command: %s", args[0]))
	}
}
