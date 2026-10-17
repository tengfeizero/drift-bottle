// Package auth 提供密码加盐哈希、账户 ID 生成与 JWT 签发/校验。
package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"time"
	"unicode/utf8"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/argon2"
)

// TokenTTL 登录凭证有效期
const TokenTTL = 7 * 24 * time.Hour

// 密码长度限制（字符数）
const (
	passwordMinLen = 6
	passwordMaxLen = 32
)

// argon2id 参数
const (
	argon2Time    = 2
	argon2Memory  = 19 * 1024 // KiB
	argon2Threads = 1
	argon2KeyLen  = 32
)

// ErrPasswordWeak 密码不符合强度要求
var ErrPasswordWeak = errors.New("密码不符合要求")

// ValidatePassword 校验密码：6–32 位，且至少包含一个字母与一个数字。
func ValidatePassword(pw string) error {
	n := utf8.RuneCountInString(pw)
	if n < passwordMinLen || n > passwordMaxLen {
		return ErrPasswordWeak
	}
	var hasLetter, hasDigit bool
	for _, r := range pw {
		switch {
		case (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z'):
			hasLetter = true
		case r >= '0' && r <= '9':
			hasDigit = true
		}
	}
	if !hasLetter || !hasDigit {
		return ErrPasswordWeak
	}
	return nil
}

// HashPassword 以 argon2id 加盐哈希，返回十六进制编码的 hash 与 salt。
func HashPassword(password string) (hash, salt string, err error) {
	s := make([]byte, 16)
	if _, err = rand.Read(s); err != nil {
		return "", "", err
	}
	h := argon2.IDKey([]byte(password), s, argon2Time, argon2Memory, argon2Threads, argon2KeyLen)
	return hex.EncodeToString(h), hex.EncodeToString(s), nil
}

// VerifyPassword 校验密码与已存 hash/salt 是否匹配。
func VerifyPassword(password, hashHex, saltHex string) bool {
	want, err1 := hex.DecodeString(hashHex)
	s, err2 := hex.DecodeString(saltHex)
	if err1 != nil || err2 != nil {
		return false
	}
	got := argon2.IDKey([]byte(password), s, argon2Time, argon2Memory, argon2Threads, argon2KeyLen)
	return subtle.ConstantTimeCompare(want, got) == 1
}

// GenerateID 生成 8 位数字 ID（首位非 0，范围 10000000–99999999）。
func GenerateID() (string, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(90000000))
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%08d", n.Int64()+10000000), nil
}

// Claims JWT 载荷：除标准字段外携带令牌版本号，用于服务端失效。
type Claims struct {
	TokenVersion int `json:"tv"`
	jwt.RegisteredClaims
}

// SignToken 签发登录凭证。
func SignToken(secret []byte, userID string, tokenVersion int, now time.Time) (string, error) {
	claims := Claims{
		TokenVersion: tokenVersion,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(TokenTTL)),
		},
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(secret)
}

// ParseToken 解析并校验登录凭证。
func ParseToken(secret []byte, tokenStr string) (*Claims, error) {
	claims := &Claims{}
	_, err := jwt.ParseWithClaims(tokenStr, claims, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("非预期的签名算法: %v", t.Header["alg"])
		}
		return secret, nil
	})
	if err != nil {
		return nil, err
	}
	return claims, nil
}
