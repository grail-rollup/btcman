package indexer

import (
	"encoding/binary"
	"encoding/hex"
	"fmt"
)

type Transaction struct {
	TxHash string `json:"tx_hash"`
	Height int32  `json:"height"`
}

type UTXO struct {
	TxPos  int    `json:"tx_pos"`
	Value  int64  `json:"value"`
	TxHash string `json:"tx_hash"`
	Height int    `json:"height"`
}

type BlockChainInfo struct {
	Height int32  `json:"height"`
	Hex    string `json:"hex"`
}

type TxInfo struct {
	Height int32  `json:"height"`
	TxHash string `json:"tx_hash"`
	Fee    int32  `json:"fee"`
}

type BlockHeader struct {
	Version      uint32
	PreviousHash string
	MerkleRoot   string
	Time         uint32
	Bits         uint32
	Nonce        uint32
}

func (b *BlockHeader) ToString() string {
	return fmt.Sprintf("Version: %d\nPreviousHash: %s\nMerkleRoot: %s\nTime: %d\nBits: %d\nNonce: %d\n", b.Version, b.PreviousHash, b.MerkleRoot, b.Time, b.Bits, b.Nonce)
}

type BlockHeaderHex string

func (b BlockHeaderHex) ToHeader() (*BlockHeader, error) {
	blockHeaderBytes, err := hex.DecodeString(string(b))
	if err != nil {
		return nil, err
	}
	return &BlockHeader{
		Version:      binary.BigEndian.Uint32(blockHeaderBytes[0:4]),
		PreviousHash: hex.EncodeToString(blockHeaderBytes[4:36]),
		MerkleRoot:   hex.EncodeToString(blockHeaderBytes[36:68]),
		Time:         binary.BigEndian.Uint32(blockHeaderBytes[68:72]),
		Bits:         binary.BigEndian.Uint32(blockHeaderBytes[72:76]),
		Nonce:        binary.BigEndian.Uint32(blockHeaderBytes[76:80]),
	}, nil
}
