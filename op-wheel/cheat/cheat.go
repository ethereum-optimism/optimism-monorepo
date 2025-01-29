package cheat

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"math/big"
	"path/filepath"
	"strings"

	"github.com/ethereum/go-ethereum/core/tracing"
	"github.com/holiman/uint256"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/consensus/beacon"
	"github.com/ethereum/go-ethereum/consensus/ethash"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/rawdb"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/ethdb/leveldb"
	"github.com/ethereum/go-ethereum/params"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/ethereum/go-ethereum/trie"

	"github.com/ethereum-optimism/optimism/op-service/eth"
)

var HundredETH = big.NewInt(0).Mul(big.NewInt(100), big.NewInt(params.Ether))

type Cheater struct {
	// The database of the chain with the head block that we patch the state-root of, once the state is updated.
	DB ethdb.Database
	// Initialized chain, wrapping the database with in-memory presentation of headers and recent changes and such.
	Blockchain *core.BlockChain
	// The Cheater avoids making writes if this is set to True, and opens the DB as readonly.
	ReadOnly bool
}

func OpenGethRawDB(dataDirPath string, readOnly bool) (ethdb.Database, error) {
	// don't use readonly mode in actual DB, it doesn't work with Geth.
	kvs, err := leveldb.New(dataDirPath, 2048, 500, "", readOnly)
	if err != nil {
		return nil, fmt.Errorf("failed to open leveldb: %w", err)
	}
	db, err := rawdb.NewDatabaseWithFreezer(kvs, filepath.Join(dataDirPath, "ancient"), "", readOnly)
	if err != nil {
		return nil, fmt.Errorf("failed to open db with freezer: %w", err)
	}
	return db, nil
}

// OpenGethDB opens a geth database to apply cheats to.
func OpenGethDB(dataDirPath string, readOnly bool) (*Cheater, error) {
	db, err := OpenGethRawDB(dataDirPath, readOnly)
	if err != nil {
		return nil, err
	}
	ch, err := core.NewBlockChain(db, nil, nil, nil,
		beacon.New(ethash.NewFullFaker()), vm.Config{}, nil)
	if err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("failed to open blockchain around chain db: %w", err)
	}
	return &Cheater{
		DB:         db,
		Blockchain: ch,
		ReadOnly:   readOnly,
	}, nil
}

func (ch *Cheater) Close() error {
	return ch.DB.Close()
}

type HeadFn func(header *types.Header, headState *state.StateDB) error

// RunAndClose runs the given function on the head-state, and then persists any changes (if not ReadOnly),
// and updates the blockchain headers indexes to reflect the new state-root, so geth will believe the cheat
// (unless it ever re-applies the block).
func (ch *Cheater) RunAndClose(fn HeadFn) error {
	preHeader := ch.Blockchain.CurrentBlock()
	if a, b := preHeader.Number.Uint64(), ch.Blockchain.Genesis().NumberU64(); a <= b {
		return fmt.Errorf("cheating at genesis (head block %d <= genesis block %d) is not supported", a, b)
	}
	state, err := ch.Blockchain.StateAt(preHeader.Root)
	if err != nil {
		_ = ch.Close()
		return fmt.Errorf("failed to look up head state: %w", err)
	}
	if err := fn(preHeader, state); err != nil {
		_ = ch.Close()
		return fmt.Errorf("failed to run state change: %w", err)
	}
	if ch.ReadOnly {
		return ch.Close()
	}

	isCancun := ch.Blockchain.Config().IsCancun(preHeader.Number, preHeader.Time)
	// commit the changes, and then update the state-root
	stateRoot, err := state.Commit(preHeader.Number.Uint64()+1, true, isCancun)
	if err != nil {
		_ = ch.Close()
		return fmt.Errorf("failed to commit state change: %w", err)
	}
	header := types.CopyHeader(preHeader) // copy the header
	header.Root = stateRoot
	blockHash := header.Hash()

	// We have to manually commit the updated state root to the database.
	if err := state.Database().TrieDB().Commit(stateRoot, true); err != nil {
		return fmt.Errorf("error committing trie db: %w", err)
	}

	// based on core.BlockChain.writeHeadBlock:
	// Add the block to the canonical chain number scheme and mark as the head
	batch := ch.DB.NewBatch()
	preID := eth.BlockID{Hash: preHeader.Hash(), Number: preHeader.Number.Uint64()}
	if ch.Blockchain.CurrentFinalBlock().Hash() == preID.Hash {
		rawdb.WriteFinalizedBlockHash(batch, blockHash)
	}
	rawdb.DeleteHeaderNumber(batch, preHeader.Hash())
	rawdb.WriteHeadHeaderHash(batch, blockHash)
	rawdb.WriteHeadFastBlockHash(batch, blockHash)
	rawdb.WriteCanonicalHash(batch, blockHash, preID.Number)
	rawdb.WriteHeaderNumber(batch, blockHash, preID.Number)
	rawdb.WriteHeader(batch, header)
	// not keyed by blockhash, and we didn't remove any txs, so we just leave this one as-is.
	// rawdb.WriteTxLookupEntriesByBlock(batch, block)
	rawdb.WriteHeadBlockHash(batch, blockHash)

	// Geth stores the TD for each block separately from the block itself. We must update this
	// manually, otherwise Geth thinks we haven't reached TTD yet and tries to build a block
	// using pre-merge consensus, which causes a panic.
	rawdb.WriteTd(batch, blockHash, preID.Number, ch.Blockchain.GetTd(preID.Hash, preID.Number))

	// Need to copy over receipts since they are keyed by block hash.
	receipts := rawdb.ReadReceipts(ch.DB, preID.Hash, preID.Number, preHeader.Time, ch.Blockchain.Config())
	rawdb.WriteReceipts(batch, blockHash, preID.Number, receipts)

	// Geth maintains an internal mapping between block bodies and their hashes. None of the database
	// accessors above update this mapping, so we need to do it manually.
	oldKey := blockBodyKey(preID.Number, preID.Hash)
	oldBody := rawdb.ReadBodyRLP(ch.DB, preID.Hash, preID.Number)
	newKey := blockBodyKey(preID.Number, blockHash)
	if err := batch.Delete(oldKey); err != nil {
		return fmt.Errorf("error deleting old block body key")
	}
	if err := batch.Put(newKey, oldBody); err != nil {
		return fmt.Errorf("error setting new block body key")
	}

	// Flush the whole batch into the disk, exit the node if failed
	if err := batch.Write(); err != nil {
		_ = ch.Close()
		return fmt.Errorf("failed to update chain indexes and markers: %w", err)
	}
	// Technically there are more in-memory things to update in real geth,
	// to which we don't even have public API access, but that's fine, we're done.
	// *And we did update the finalized marker, which is flushed from memory to disk on shutdown in geth.
	// bc.hc.SetCurrentHeader(block.Header())
	// headFastBlockGauge.Update(int64(block.NumberU64()))
	// headBlockGauge.Update(int64(block.NumberU64()))

	return ch.Close()
}

// StorageSet modifies the storage of the given address at the given key to the given value.
func StorageSet(address common.Address, key common.Hash, value common.Hash) HeadFn {
	return func(_ *types.Header, headState *state.StateDB) error {
		headState.SetState(address, key, value)
		return nil
	}
}

// StorageGet just reads the storage of the given address at the given key.
func StorageGet(address common.Address, key common.Hash, w io.Writer) HeadFn {
	return func(_ *types.Header, headState *state.StateDB) error {
		value := headState.GetState(address, key)
		_, err := io.WriteString(w, value.Hex())
		return err
	}
}

// StorageReadAll reads all values of the given address, and writes it as a (+) diff to the given output writer.
// Simply replace the (+) with (-) if you need to apply the diff as removal of values.
// Combined with StoragePatch this allows for quick surgery of 1 account in one database,
// to another account (maybe even in a different database!).
func StorageReadAll(address common.Address, w io.Writer) HeadFn {
	return func(_ *types.Header, headState *state.StateDB) error {
		storage, err := headState.OpenStorageTrie(address)
		if err != nil {
			return fmt.Errorf("failed to open storage trie of addr %s: %w", address, err)
		}
		if storage == nil {
			return fmt.Errorf("no storage trie in state for account %s", address)
		}
		nodeIter, err := storage.NodeIterator(nil)
		if err != nil {
			return fmt.Errorf("failed to create node iterator for storage of %s: %w", address, err)
		}
		iter := trie.NewIterator(nodeIter)
		for iter.Next() {
			if _, err := fmt.Fprintf(w, "+ %x = %x\n", iter.Key, dbValueToHash(iter.Value)); err != nil {
				return err
			}
		}
		return nil
	}
}

func dbValueToHash(enc []byte) common.Hash {
	var value common.Hash
	if len(enc) > 0 {
		_, content, _, err := rlp.Split(enc)
		if err != nil {
			panic(err)
		}
		value.SetBytes(content)
	}
	return value
}

// StorageDiff compares the storage of two different accounts, and writes a patch with differences.
// Each difference is expressed with 1 character + or - to indicate the change from a to b, followed by key = value.
func StorageDiff(out io.Writer, addressA, addressB common.Address) HeadFn {
	return func(_ *types.Header, headState *state.StateDB) error {
		aStorage, err := headState.OpenStorageTrie(addressA)
		if err != nil {
			return fmt.Errorf("failed to open storage trie of addr A %s: %w", addressA, err)
		}
		if aStorage == nil {
			return fmt.Errorf("no storage trie in state for account A %s", addressA)
		}
		bStorage, err := headState.OpenStorageTrie(addressB)
		if err != nil {
			return fmt.Errorf("failed to open storage trie of addr B %s: %w", addressB, err)
		}
		if bStorage == nil {
			return fmt.Errorf("no storage trie in state for account B %s", addressB)
		}
		aNodeIter, err := aStorage.NodeIterator(nil)
		if err != nil {
			return fmt.Errorf("failed to create node iterator for storage of %s (A): %w", addressA, err)
		}
		bNodeIter, err := bStorage.NodeIterator(nil)
		if err != nil {
			return fmt.Errorf("failed to create node iterator for storage of %s (b): %w", addressB, err)
		}
		aIter := trie.NewIterator(aNodeIter)
		bIter := trie.NewIterator(bNodeIter)
		hasA := aIter.Next()
		hasB := bIter.Next()
		for {
			if !hasA && !hasB {
				break
			}
			if cmp := bytes.Compare(aIter.Key, bIter.Key); cmp < 0 {
				// a is smaller, and thus missing in b. Print and move forward a
				if _, err := fmt.Fprintf(out, "- %x = %x\n", aIter.Key, dbValueToHash(aIter.Value)); err != nil {
					return err
				}
				hasA = aIter.Next()
			} else if cmp > 0 {
				// b is smaller, and thus missing in a. Print and move forward b
				if _, err := fmt.Fprintf(out, "+ %x = %x\n", bIter.Key, dbValueToHash(bIter.Value)); err != nil {
					return err
				}
				hasB = bIter.Next()
			} else if cmp == 0 {
				// same key, now check if the values differ
				if !bytes.Equal(aIter.Value, bIter.Value) {
					if _, err := fmt.Fprintf(out, "- %x = %x\n", aIter.Key, dbValueToHash(aIter.Value)); err != nil {
						return err
					}
					if _, err := fmt.Fprintf(out, "+ %x = %x\n", bIter.Key, dbValueToHash(bIter.Value)); err != nil {
						return err
					}
				}
				// move both
				hasA = aIter.Next()
				hasB = bIter.Next()
			}
		}
		return nil
	}
}

// StoragePatch applies a patch of changes to the given state account trie.
// Changes are hex encoded key-value pairs separated by (=).
// Additions are prefixed with (+).
// Deletions are prefixed with (-) and overwrite it to a zero value.
// Comments (#) and empty lines are ignored.
func StoragePatch(patch io.Reader, address common.Address) HeadFn {
	return func(head *types.Header, headState *state.StateDB) error {
		s := bufio.NewScanner(patch)
		i := 0
		for s.Scan() {
			line := s.Text()
			if len(line) < 1 || line[0] == '#' { // skip empty lines and comments
				continue
			}
			parts := strings.Split(line[1:], "=")
			keyHex := strings.TrimSpace(parts[0])
			valueHex := strings.TrimSpace(parts[1])
			var key, value common.Hash
			if err := key.UnmarshalText([]byte(keyHex)); err != nil {
				return fmt.Errorf("key %s is malformatted: %w", keyHex, err)
			}
			if err := value.UnmarshalText([]byte(valueHex)); err != nil {
				return fmt.Errorf("key %s has malformatted value %s: %w", keyHex, valueHex, err)
			}
			switch line[0] {
			case '+':
				headState.SetState(address, key, value)
			case '-':
				headState.SetState(address, key, common.Hash{})
			default:
				return fmt.Errorf("unrecognized line diff token")
			}
			i += 1
			if i%1000 == 0 { // for every 1000 values, commit to disk
				// warning: if the account is empty, the storage change will not persist.
				if _, err := headState.Commit(head.Number.Uint64(), true, false); err != nil {
					return fmt.Errorf("failed to commit state to disk after patching %d entries: %w", i, err)
				}
			}
		}
		return nil
	}
}

func SetBalance(addr common.Address, amount *big.Int) HeadFn {
	return func(_ *types.Header, headState *state.StateDB) error {
		headState.SetBalance(addr, uint256.MustFromBig(amount), tracing.BalanceChangeUnspecified)
		return nil
	}
}

func SetCode(addr common.Address, code hexutil.Bytes) HeadFn {
	return func(_ *types.Header, headState *state.StateDB) error {
		headState.SetCode(addr, code)
		return nil
	}
}

func SetNonce(addr common.Address, nonce uint64) HeadFn {
	return func(_ *types.Header, headState *state.StateDB) error {
		headState.SetNonce(addr, nonce)
		return nil
	}
}

// blockBodyKey returns the database key to use for storing the body of a block.
// This function was copied from Geth's core/rawdb/accessors_chain.go.
func blockBodyKey(number uint64, hash common.Hash) []byte {
	return append(append([]byte("b"), encodeBlockNumber(number)...), hash.Bytes()...)
}

// encodeBlockNumber encodes a block number as big endian uint64. This function was
// copied from Geth's core/rawdb/accessors_chain.go file.
func encodeBlockNumber(number uint64) []byte {
	enc := make([]byte, 8)
	binary.BigEndian.PutUint64(enc, number)
	return enc
}
