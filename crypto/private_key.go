package crypto

import (
	"encoding/hex"
	"log"

	"github.com/ethereum/go-ethereum/crypto/secp256k1"
)

type PrivateKey struct {
	key []byte
}

func (pk *PrivateKey) FromHexStr(hs string) *PrivateKey {
	h, _ := hex.DecodeString(hs)
	pk.key = h
	return pk
}

func (pk *PrivateKey) Bytes() []byte {
	return pk.key
}

func (pk *PrivateKey) Sign(msg []byte) []byte {
	s, err := secp256k1.Sign(msg, pk.Bytes())
	if err != nil {
		log.Printf("[CRYPTO SECP256K1] sign error ERROR=%v\n", err)
		return nil
	}
	result := compactSig(s)
	return result
}

func compactSig(s []byte) []byte {
	result := make([]byte, 1, 65)
	result[0] = 27 + s[64] + 4
	result = append(result, s[0:32]...)
	result = append(result, s[32:64]...)
	return result
}
