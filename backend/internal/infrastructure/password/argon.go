package password

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"strings"

	"golang.org/x/crypto/argon2"
)

const prefix = "$argon2id$v=19$m=65536,t=3,p=1$"

type Argon struct{}

func (Argon) Hash(password string) (string, error) {
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	key := argon2.IDKey([]byte(password), salt, 3, 64*1024, 1, 32)
	return prefix + base64.RawStdEncoding.EncodeToString(salt) + "$" + base64.RawStdEncoding.EncodeToString(key), nil
}
func (Argon) Verify(encoded, password string) bool {
	salt, expected, valid := decode(encoded)
	actual := argon2.IDKey([]byte(password), salt, 3, 64*1024, 1, 32)
	return subtle.ConstantTimeCompare(actual, expected) == 1 && valid
}
func decode(encoded string) (salt, key []byte, valid bool) {
	parts := strings.Split(strings.TrimPrefix(encoded, prefix), "$")
	if strings.HasPrefix(encoded, prefix) && len(parts) == 2 {
		salt, saltErr := base64.RawStdEncoding.DecodeString(parts[0])
		key, keyErr := base64.RawStdEncoding.DecodeString(parts[1])
		if saltErr == nil && keyErr == nil && len(salt) == 16 && len(key) == 32 {
			return salt, key, true
		}
	}
	return make([]byte, 16), make([]byte, 32), false
}
