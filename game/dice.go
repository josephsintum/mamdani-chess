package game

import (
	"crypto/rand"
	"math/big"
)

// codeAlphabet has no lookalikes: no 0, O, 1, I or L.
const codeAlphabet = "ABCDEFGHJKMNPQRSTUVWXYZ23456789"

// NewCode returns a random 6-character game code such as "K7F3QZ".
func NewCode() string {
	b := make([]byte, 6)
	for i := range b {
		b[i] = codeAlphabet[randN(len(codeAlphabet))]
	}
	return string(b)
}

// CryptoDice rolls fair d8s from crypto/rand.
type CryptoDice struct{}

func (CryptoDice) D8() int { return randN(8) + 1 }

func randN(n int) int {
	v, err := rand.Int(rand.Reader, big.NewInt(int64(n)))
	if err != nil {
		panic(err) // crypto/rand never fails on supported platforms
	}
	return int(v.Int64())
}
