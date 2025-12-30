package totp

import (
	"crypto/hmac"
	"crypto/sha1"
	"encoding/base32"
	"encoding/binary"
	"strconv"
	"strings"
	"time"
)

type TOTPGenerator struct {
	Secret string
	Digits int
	Period int64
}

func (t TOTPGenerator) String() string {
	return "TOTP"
}

func (t *TOTPGenerator) Generate() (string, error) {
	// Normalize and decode Base32 secret
	secretUpper := strings.ToUpper(strings.ReplaceAll(t.Secret, " ", ""))
	key, err := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(secretUpper)
	if err != nil {
		return "", err
	}

	// Time counter = Unix time / period
	counter := time.Now().Unix() / t.Period

	// Convert counter to 8-byte big-endian
	var buf [8]byte
	binary.BigEndian.PutUint64(buf[:], uint64(counter))

	// HMAC-SHA1
	h := hmac.New(sha1.New, key)
	h.Write(buf[:])
	hash := h.Sum(nil)

	// Dynamic truncation
	offset := hash[len(hash)-1] & 0x0f
	code := (int(hash[offset])&0x7f)<<24 |
		(int(hash[offset+1])&0xff)<<16 |
		(int(hash[offset+2])&0xff)<<8 |
		(int(hash[offset+3]) & 0xff)

	// Mod 10^digits
	modulo := 1
	for range t.Digits {
		modulo *= 10
	}
	otp := code % modulo

	return zeropad(strconv.FormatInt(int64(otp), 10), t.Digits), nil
}

func zeropad(input string, length int) string {
	for len(input) < length {
		input = "0" + input
	}
	return input
}
