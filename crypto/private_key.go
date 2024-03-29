package crypto

import (
	"encoding/hex"
	"errors"
	"math/big"

	"github.com/ethereum/go-ethereum/common/math"
	"github.com/ethereum/go-ethereum/crypto/secp256k1"
	log "github.com/inconshreveable/log15"
)

type PrivateKey struct {
	nodeId string
	seckey []byte
}

func (pk *PrivateKey) SetKey(keyHexStr string) error {
	d, ok := new(big.Int).SetString(keyHexStr, 16)
	if ok == false {
		return errors.New("parse hex string failed")
	}
	curve := secp256k1.S256()
	// seckey
	pk.seckey = math.PaddedBigBytes(d, curve.Params().BitSize/8)

	// nodeId
	x, y := curve.ScalarBaseMult(pk.seckey)
	xs := x.Bytes()
	if len(xs) != 32 {
		return errors.New("incorrect public key")
	}
	ret0 := uint8(3)
	if y.Bit(0) == 0 {
		ret0 = 2
	}
	nodeId := make([]byte, 33)
	nodeId[0] = ret0
	copy(nodeId[1:], xs)
	nodeId = Ripemd160Sha256(nodeId)
	pk.nodeId = hex.EncodeToString(nodeId)

	return nil
}

func (pk *PrivateKey) NodeId() string {
	return pk.nodeId
}

func (pk *PrivateKey) Sign(hash []byte) []byte {
	s, err := secp256k1.Sign(hash, pk.seckey)
	if err != nil {
		log.Warn("sign error", "subject", "secp256k1", "error", err)
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
