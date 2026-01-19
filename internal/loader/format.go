package loader

import (
	"bytes"
	"crypto/ed25519"
	"encoding/binary"
	"fmt"
	"io"
)

const (
	PPKMagic    = "PPK\x00"
	PPKVersion  = 0x01
	AlgoEd25519 = 0x01
	HeaderSize  = 128
)

// PpkHeader PPK 文件头结构
type PpkHeader struct {
	Magic      [4]byte
	Version    uint8
	Flags      uint8
	SignAlgo   uint8
	Reserved1  uint8
	ContentLen uint64
	PublicKey  [32]byte
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
	// 0x00-0x03: MAGIC
	copy(header.Magic[:], buf[0:4])
	if string(header.Magic[:]) != PPKMagic {
		return nil, fmt.Errorf("invalid ppk magic")
	}

	// 0x04: VERSION
	header.Version = buf[4]
	if header.Version != PPKVersion {
		return nil, fmt.Errorf("unsupported ppk version: %d", header.Version)
	}

	// 0x05: FLAGS
	header.Flags = buf[5]

	// 0x06: SIGN_ALGO
	header.SignAlgo = buf[6]
	if header.SignAlgo != AlgoEd25519 {
		return nil, fmt.Errorf("unsupported signature algorithm: %d", header.SignAlgo)
	}

	// 0x07: RESERVED1
	header.Reserved1 = buf[7]

	// 0x08-0x0F: CONTENT_LEN
	header.ContentLen = binary.LittleEndian.Uint64(buf[8:16])

	// 0x10-0x2F: PUBLIC_KEY
	copy(header.PublicKey[:], buf[16:48])

	// 0x30-0x6F: SIGNATURE
	copy(header.Signature[:], buf[48:112])

	// 0x70-0x7F: RESERVED2
	copy(header.Reserved2[:], buf[112:128])

	return header, nil
}

// VerifySignature 验证数据签名
func (h *PpkHeader) VerifySignature(data []byte, trustedPubKey ed25519.PublicKey) error {
	// 1. 如果传入了受信任的公钥（如来自 release.pub），先验证包头里的公钥是否匹配
	if len(trustedPubKey) > 0 {
		if !bytes.Equal(h.PublicKey[:], trustedPubKey) {
			return fmt.Errorf("public key mismatch: expected %x, got %x", trustedPubKey, h.PublicKey[:])
		}
	}

	// 2. 使用包头里的公钥验证签名
	if !ed25519.Verify(h.PublicKey[:], data, h.Signature[:]) {
		return fmt.Errorf("signature verification failed")
	}
	return nil
}
