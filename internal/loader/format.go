package loader

import (
	"crypto/ed25519"
	"encoding/binary"
	"fmt"
	"io"
)

const (
	PPKMagic    = "PPK\x00"
	PPKVersion  = 0x01
	AlgoEd25519 = 0x01
	HeaderSize  = 96
)

// PpkHeader PPK 文件头结构
type PpkHeader struct {
	Magic      [4]byte
	Version    uint8
	Flags      uint8
	SignAlgo   uint8
	Reserved1  uint8
	ContentLen uint64
	Signature  [64]byte
	Reserved2  [16]byte
}

// ParsePpkHeader 解析 PPK 文件头
func ParsePpkHeader(r io.Reader) (*PpkHeader, error) {
	buf := make([]byte, HeaderSize)
	if _, err := io.ReadFull(r, buf); err != nil {
		return nil, fmt.Errorf("failed to read ppk header: %w", err)
	}

	header := &PpkHeader{}
	copy(header.Magic[:], buf[0:4])
	if string(header.Magic[:]) != PPKMagic {
		return nil, fmt.Errorf("invalid ppk magic")
	}

	header.Version = buf[4]
	if header.Version != PPKVersion {
		return nil, fmt.Errorf("unsupported ppk version: %d", header.Version)
	}

	header.Flags = buf[5]
	header.SignAlgo = buf[6]
	if header.SignAlgo != AlgoEd25519 {
		return nil, fmt.Errorf("unsupported signature algorithm: %d", header.SignAlgo)
	}

	header.ContentLen = binary.LittleEndian.Uint64(buf[8:16])
	copy(header.Signature[:], buf[16:80])

	// buf[80:96] is reserved

	return header, nil
}

// VerifySignature 验证数据签名
func (h *PpkHeader) VerifySignature(data []byte, pubKey ed25519.PublicKey) error {
	if !ed25519.Verify(pubKey, data, h.Signature[:]) {
		return fmt.Errorf("signature verification failed")
	}
	return nil
}
