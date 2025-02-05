package derive

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/holiman/uint256"
)

type spanBatchTxs struct {
	// this field must be manually set
	totalBlockTxCount uint64

	// 8 fields
	contractCreationBits *big.Int // standard span-batch bitlist
	yParityBits          *big.Int // standard span-batch bitlist
	txSigs               []spanBatchSignature
	txNonces             []uint64
	txGases              []uint64
	txTos                []common.Address
	txDatas              []hexutil.Bytes
	protectedBits        *big.Int // standard span-batch bitlist

	// intermediate variables which can be recovered
	txTypes            []int
	totalLegacyTxCount uint64
}

type spanBatchSignature struct {
	v uint64
	r *uint256.Int
	s *uint256.Int
}

func (btx *spanBatchTxs) encodeContractCreationBits(w io.Writer) error {
	if err := encodeSpanBatchBits(w, btx.totalBlockTxCount, btx.contractCreationBits); err != nil {
		return fmt.Errorf("failed to encode contract creation bits: %w", err)
	}
	return nil
}

func (btx *spanBatchTxs) decodeContractCreationBits(r *bytes.Reader) error {
	if btx.totalBlockTxCount > MaxSpanBatchElementCount {
		return ErrTooBigSpanBatchSize
	}
	bits, err := decodeSpanBatchBits(r, btx.totalBlockTxCount)
	if err != nil {
		return fmt.Errorf("failed to decode contract creation bits: %w", err)
	}
	btx.contractCreationBits = bits
	return nil
}

func (btx *spanBatchTxs) encodeProtectedBits(w io.Writer) error {
	if err := encodeSpanBatchBits(w, btx.totalLegacyTxCount, btx.protectedBits); err != nil {
		return fmt.Errorf("failed to encode protected bits: %w", err)
	}
	return nil
}

func (btx *spanBatchTxs) decodeProtectedBits(r *bytes.Reader) error {
	if btx.totalLegacyTxCount > MaxSpanBatchElementCount {
		return ErrTooBigSpanBatchSize
	}
	bits, err := decodeSpanBatchBits(r, btx.totalLegacyTxCount)
	if err != nil {
		return fmt.Errorf("failed to decode protected bits: %w", err)
	}
	btx.protectedBits = bits
	return nil
}

func (btx *spanBatchTxs) contractCreationCount() (uint64, error) {
	if btx.contractCreationBits == nil {
		return 0, errors.New("dev error: contract creation bits not set")
	}
	var result uint64 = 0
	for i := 0; i < int(btx.totalBlockTxCount); i++ {
		bit := btx.contractCreationBits.Bit(i)
		if bit == 1 {
			result++
		}
	}
	return result, nil
}

func (btx *spanBatchTxs) encodeYParityBits(w io.Writer) error {
	if err := encodeSpanBatchBits(w, btx.totalBlockTxCount, btx.yParityBits); err != nil {
		return fmt.Errorf("failed to encode y-parity bits: %w", err)
	}
	return nil
}

func (btx *spanBatchTxs) decodeYParityBits(r *bytes.Reader) error {
	bits, err := decodeSpanBatchBits(r, btx.totalBlockTxCount)
	if err != nil {
		return fmt.Errorf("failed to decode y-parity bits: %w", err)
	}
	btx.yParityBits = bits
	return nil
}

func (btx *spanBatchTxs) encodeTxSigsRS(w io.Writer) error {
	for _, txSig := range btx.txSigs {
		rBuf := txSig.r.Bytes32()
		if _, err := w.Write(rBuf[:]); err != nil {
			return fmt.Errorf("cannot write tx sig r: %w", err)
		}
		sBuf := txSig.s.Bytes32()
		if _, err := w.Write(sBuf[:]); err != nil {
			return fmt.Errorf("cannot write tx sig s: %w", err)
		}
	}
	return nil
}

func (btx *spanBatchTxs) encodeTxNonces(w io.Writer) error {
	var buf [binary.MaxVarintLen64]byte
	for _, txNonce := range btx.txNonces {
		n := binary.PutUvarint(buf[:], txNonce)
		if _, err := w.Write(buf[:n]); err != nil {
			return fmt.Errorf("cannot write tx nonce: %w", err)
		}
	}
	return nil
}

func (btx *spanBatchTxs) encodeTxGases(w io.Writer) error {
	var buf [binary.MaxVarintLen64]byte
	for _, txGas := range btx.txGases {
		n := binary.PutUvarint(buf[:], txGas)
		if _, err := w.Write(buf[:n]); err != nil {
			return fmt.Errorf("cannot write tx gas: %w", err)
		}
	}
	return nil
}

func (btx *spanBatchTxs) encodeTxTos(w io.Writer) error {
	for _, txTo := range btx.txTos {
		if _, err := w.Write(txTo.Bytes()); err != nil {
			return fmt.Errorf("cannot write tx to address: %w", err)
		}
	}
	return nil
}

func (btx *spanBatchTxs) encodeTxDatas(w io.Writer) error {
	for _, txData := range btx.txDatas {
		if _, err := w.Write(txData); err != nil {
			return fmt.Errorf("cannot write tx data: %w", err)
		}
	}
	return nil
}

func (btx *spanBatchTxs) decodeTxSigsRS(r *bytes.Reader) error {
	var txSigs []spanBatchSignature
	var sigBuffer [32]byte
	for i := 0; i < int(btx.totalBlockTxCount); i++ {
		var txSig spanBatchSignature
		_, err := io.ReadFull(r, sigBuffer[:])
		if err != nil {
			return fmt.Errorf("failed to read tx sig r: %w", err)
		}
		txSig.r, _ = uint256.FromBig(new(big.Int).SetBytes(sigBuffer[:]))
		_, err = io.ReadFull(r, sigBuffer[:])
		if err != nil {
			return fmt.Errorf("failed to read tx sig s: %w", err)
		}
		txSig.s, _ = uint256.FromBig(new(big.Int).SetBytes(sigBuffer[:]))
		txSigs = append(txSigs, txSig)
	}
	btx.txSigs = txSigs
	return nil
}

func (btx *spanBatchTxs) decodeTxNonces(r *bytes.Reader) error {
	var txNonces []uint64
	for i := 0; i < int(btx.totalBlockTxCount); i++ {
		txNonce, err := binary.ReadUvarint(r)
		if err != nil {
			return fmt.Errorf("failed to read tx nonce: %w", err)
		}
		txNonces = append(txNonces, txNonce)
	}
	btx.txNonces = txNonces
	return nil
}

func (btx *spanBatchTxs) decodeTxGases(r *bytes.Reader) error {
	var txGases []uint64
	for i := 0; i < int(btx.totalBlockTxCount); i++ {
		txGas, err := binary.ReadUvarint(r)
		if err != nil {
			return fmt.Errorf("failed to read tx gas: %w", err)
		}
		txGases = append(txGases, txGas)
	}
	btx.txGases = txGases
	return nil
}

func (btx *spanBatchTxs) decodeTxTos(r *bytes.Reader) error {
	var txTos []common.Address
	txToBuffer := make([]byte, common.AddressLength)
	contractCreationCount, err := btx.contractCreationCount()
	if err != nil {
		return err
	}
	for i := 0; i < int(btx.totalBlockTxCount-contractCreationCount); i++ {
		_, err := io.ReadFull(r, txToBuffer)
		if err != nil {
			return fmt.Errorf("failed to read tx to address: %w", err)
		}
		txTos = append(txTos, common.BytesToAddress(txToBuffer))
	}
	btx.txTos = txTos
	return nil
}

func (btx *spanBatchTxs) decodeTxDatas(r *bytes.Reader) error {
	var txDatas []hexutil.Bytes
	var txTypes []int
	// Do not need txDataHeader because RLP byte stream already includes length info
	for i := 0; i < int(btx.totalBlockTxCount); i++ {
		txData, txType, err := ReadTxData(r)
		if err != nil {
			return err
		}
		txDatas = append(txDatas, txData)
		txTypes = append(txTypes, txType)
		if txType == types.LegacyTxType {
			btx.totalLegacyTxCount++
		}
	}
	btx.txDatas = txDatas
	btx.txTypes = txTypes
	return nil
}

func (btx *spanBatchTxs) recoverV(chainID *big.Int) error {
	if len(btx.txTypes) != len(btx.txSigs) {
		return errors.New("tx type length and tx sigs length mismatch")
	}
	if btx.protectedBits == nil {
		return errors.New("dev error: protected bits not set")
	}
	protectedBitsIdx := 0
	for idx, txType := range btx.txTypes {
		bit := uint64(btx.yParityBits.Bit(idx))
		var v uint64
		switch txType {
		case types.LegacyTxType:
			protectedBit := btx.protectedBits.Bit(protectedBitsIdx)
			protectedBitsIdx++
			if protectedBit == 0 {
				v = 27 + bit
			} else {
				// EIP-155
				v = chainID.Uint64()*2 + 35 + bit
			}
		case types.AccessListTxType:
			v = bit
		case types.DynamicFeeTxType:
			v = bit
		case types.SetCodeTxType:
			v = bit
		default:
			return fmt.Errorf("invalid tx type: %d", txType)
		}
		btx.txSigs[idx].v = v
	}
	return nil
}

func (btx *spanBatchTxs) encode(w io.Writer) error {
	if err := btx.encodeContractCreationBits(w); err != nil {
		return err
	}
	if err := btx.encodeYParityBits(w); err != nil {
		return err
	}
	if err := btx.encodeTxSigsRS(w); err != nil {
		return err
	}
	if err := btx.encodeTxTos(w); err != nil {
		return err
	}
	if err := btx.encodeTxDatas(w); err != nil {
		return err
	}
	if err := btx.encodeTxNonces(w); err != nil {
		return err
	}
	if err := btx.encodeTxGases(w); err != nil {
		return err
	}
	if err := btx.encodeProtectedBits(w); err != nil {
		return err
	}
	return nil
}

func (btx *spanBatchTxs) decode(r *bytes.Reader) error {
	if err := btx.decodeContractCreationBits(r); err != nil {
		return err
	}
	if err := btx.decodeYParityBits(r); err != nil {
		return err
	}
	if err := btx.decodeTxSigsRS(r); err != nil {
		return err
	}
	if err := btx.decodeTxTos(r); err != nil {
		return err
	}
	if err := btx.decodeTxDatas(r); err != nil {
		return err
	}
	if err := btx.decodeTxNonces(r); err != nil {
		return err
	}
	if err := btx.decodeTxGases(r); err != nil {
		return err
	}
	if err := btx.decodeProtectedBits(r); err != nil {
		return err
	}
	return nil
}

func (btx *spanBatchTxs) fullTxs(chainID *big.Int) ([][]byte, error) {
	var txs [][]byte
	toIdx := 0
	for idx := 0; idx < int(btx.totalBlockTxCount); idx++ {
		var stx spanBatchTx
		if err := stx.UnmarshalBinary(btx.txDatas[idx]); err != nil {
			return nil, err
		}
		nonce := btx.txNonces[idx]
		gas := btx.txGases[idx]
		var to *common.Address = nil
		bit := btx.contractCreationBits.Bit(idx)
		if bit == 0 {
			if len(btx.txTos) <= toIdx {
				return nil, errors.New("tx to not enough")
			}
			to = &btx.txTos[toIdx]
			toIdx++
		}
		v := new(big.Int).SetUint64(btx.txSigs[idx].v)
		r := btx.txSigs[idx].r.ToBig()
		s := btx.txSigs[idx].s.ToBig()
		tx, err := stx.convertToFullTx(nonce, gas, to, chainID, v, r, s)
		if err != nil {
			return nil, err
		}
		encodedTx, err := tx.MarshalBinary()
		if err != nil {
			return nil, err
		}
		txs = append(txs, encodedTx)
	}
	return txs, nil
}

func convertVToYParity(v uint64, txType int) (uint, error) {
	var yParityBit uint
	switch txType {
	case types.LegacyTxType:
		if isProtectedV(v, txType) {
			// EIP-155: v = 2 * chainID + 35 + yParity
			// v - 35 = yParity (mod 2)
			yParityBit = uint((v - 35) & 1)
		} else {
			// unprotected legacy txs must have v = 27 or 28
			yParityBit = uint(v - 27)
		}
	case types.AccessListTxType:
		yParityBit = uint(v)
	case types.DynamicFeeTxType:
		yParityBit = uint(v)
	case types.SetCodeTxType:
		yParityBit = uint(v)
	default:
		return 0, fmt.Errorf("invalid tx type: %d", txType)
	}
	return yParityBit, nil
}

func isProtectedV(v uint64, txType int) bool {
	if txType == types.LegacyTxType {
		// if EIP-155 applied, v = 2 * chainID + 35 + yParity
		return v != 27 && v != 28
	}
	// every non legacy tx are protected
	return true
}

func newSpanBatchTxs(txs [][]byte, chainID *big.Int) (*spanBatchTxs, error) {
	sbtxs := &spanBatchTxs{
		contractCreationBits: big.NewInt(0),
		yParityBits:          big.NewInt(0),
		txSigs:               []spanBatchSignature{},
		txNonces:             []uint64{},
		txGases:              []uint64{},
		txTos:                []common.Address{},
		txDatas:              []hexutil.Bytes{},
		txTypes:              []int{},
		protectedBits:        big.NewInt(0),
	}

	if err := sbtxs.AddTxs(txs, chainID); err != nil {
		return nil, err
	}
	return sbtxs, nil
}

func (sbtx *spanBatchTxs) AddTxs(txs [][]byte, chainID *big.Int) error {
	totalBlockTxCount := uint64(len(txs))
	offset := sbtx.totalBlockTxCount
	for idx := 0; idx < int(totalBlockTxCount); idx++ {
		tx := &types.Transaction{}
		if err := tx.UnmarshalBinary(txs[idx]); err != nil {
			return errors.New("failed to decode tx")
		}
		if tx.Type() == types.LegacyTxType {
			protectedBit := uint(0)
			if tx.Protected() {
				protectedBit = uint(1)
			}
			sbtx.protectedBits.SetBit(sbtx.protectedBits, int(sbtx.totalLegacyTxCount), protectedBit)
			sbtx.totalLegacyTxCount++
		}
		if tx.Protected() && tx.ChainId().Cmp(chainID) != 0 {
			return fmt.Errorf("protected tx has chain ID %d, but expected chain ID %d", tx.ChainId(), chainID)
		}
		var txSig spanBatchSignature
		v, r, s := tx.RawSignatureValues()
		R, _ := uint256.FromBig(r)
		S, _ := uint256.FromBig(s)
		txSig.v = v.Uint64()
		txSig.r = R
		txSig.s = S
		sbtx.txSigs = append(sbtx.txSigs, txSig)
		contractCreationBit := uint(1)
		if tx.To() != nil {
			sbtx.txTos = append(sbtx.txTos, *tx.To())
			contractCreationBit = uint(0)
		}
		sbtx.contractCreationBits.SetBit(sbtx.contractCreationBits, idx+int(offset), contractCreationBit)
		yParityBit, err := convertVToYParity(txSig.v, int(tx.Type()))
		if err != nil {
			return err
		}
		sbtx.yParityBits.SetBit(sbtx.yParityBits, idx+int(offset), yParityBit)
		sbtx.txNonces = append(sbtx.txNonces, tx.Nonce())
		sbtx.txGases = append(sbtx.txGases, tx.Gas())
		stx, err := newSpanBatchTx(tx)
		if err != nil {
			return err
		}
		txData, err := stx.MarshalBinary()
		if err != nil {
			return err
		}
		sbtx.txDatas = append(sbtx.txDatas, txData)
		sbtx.txTypes = append(sbtx.txTypes, int(tx.Type()))
	}
	sbtx.totalBlockTxCount += totalBlockTxCount
	return nil
}
