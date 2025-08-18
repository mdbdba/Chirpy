package auth

import (
	"crypto/rand"
	"encoding/hex"
)

func MakeRefreshToken() (string, error) {
	key := make([]byte, 32)
	read, err := rand.Read(key)
	if err != nil {
		return "", err
	}
	returnValue := hex.EncodeToString(key[:read])
	return returnValue, nil

}
