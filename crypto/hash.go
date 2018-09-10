package crypto

import (
	"crypto/sha256"

	"golang.org/x/crypto/ripemd160"
)

func Ripemd160Sha256(data []byte) []byte {
	s256 := sha256.Sum256(data)
	hrip := ripemd160.New()
	hrip.Write(s256[:])
	ret := hrip.Sum(nil)
	return ret
}
