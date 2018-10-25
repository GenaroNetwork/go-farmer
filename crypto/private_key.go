package crypto

import (
	"crypto/ecdsa"
	"encoding/hex"
	"errors"
	"log"

	"github.com/ethereum/go-ethereum/crypto"
)

type PrivateKey struct {
	Key *ecdsa.PrivateKey
}

func (pk *PrivateKey) NodeId() (string, error) {
	xs := pk.Key.PublicKey.X.Bytes()
	if len(xs) != 32 {
		return "", errors.New("incorrect public key")
	}
	ret0 := uint8(3)
	if pk.Key.PublicKey.Y.Bit(0) == 0 {
		ret0 = 2
	}
	nodeId := make([]byte, 33)
	nodeId[0] = ret0
	copy(nodeId[1:], xs)
	nodeId = Ripemd160Sha256(nodeId)
	return hex.EncodeToString(nodeId), nil
}

func (pk *PrivateKey) Sign(hash []byte) []byte {
	s, err := crypto.Sign(hash, pk.Key)
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
