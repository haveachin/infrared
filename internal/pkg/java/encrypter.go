package java

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"errors"
)

const (
	keyBitSize        = 1024
	verifyTokenLength = 4
)

type SessionEncrypter interface {
	GenerateVerifyToken() ([]byte, error)
	DecryptAndVerifySharedSecret(verifyToken, encVerifyToken, encSharedSecret []byte) ([]byte, error)
}

type defaultSessionEncrypter struct {
	privKey *rsa.PrivateKey
}

func NewDefaultSessionEncrypter() (SessionEncrypter, []byte, error) {
	key, err := rsa.GenerateKey(rand.Reader, keyBitSize)
	if err != nil {
		return nil, nil, err
	}

	pubKey, err := x509.MarshalPKIXPublicKey(&key.PublicKey)
	if err != nil {
		return nil, nil, err
	}

	return &defaultSessionEncrypter{
		privKey: key,
	}, pubKey, nil
}

func (enc *defaultSessionEncrypter) GenerateVerifyToken() ([]byte, error) {
	verifyToken := make([]byte, verifyTokenLength)
	if _, err := rand.Read(verifyToken); err != nil {
		return nil, err
	}

	return verifyToken, nil
}

func (enc *defaultSessionEncrypter) DecryptAndVerifySharedSecret(verifyToken, encVerifyToken, encSharedSecret []byte) ([]byte, error) {
	decVerifyToken, err := enc.privKey.Decrypt(rand.Reader, encVerifyToken, nil)
	if err != nil {
		return nil, err
	}

	if !bytes.Equal(verifyToken, decVerifyToken) {
		return nil, errors.New("verify token did not match")
	}

	return enc.privKey.Decrypt(rand.Reader, encSharedSecret, nil)
}
