package session

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"errors"
	"sync"
)

const (
	keyBitSize        = 1024
	verifyTokenLength = 4
)

type SessionEncrypter interface {
	PublicKey() []byte
	GenerateVerifyToken() ([]byte, error)
	DecryptAndVerifySharedSecret(r *Request, sharedSecret, verifyToken []byte) ([]byte, error)
}

type DefaultSessionEncrypter struct {
	privateKey *rsa.PrivateKey

	mu           sync.Mutex
	publicKey    []byte
	verifyTokens map[*conn][]byte
}

func NewDefaultSessionEncrypter() (SessionEncrypter, error) {
	key, err := rsa.GenerateKey(rand.Reader, keyBitSize)
	if err != nil {
		return nil, err
	}

	publicKey, err := x509.MarshalPKIXPublicKey(&key.PublicKey)
	if err != nil {
		return nil, err
	}

	return &DefaultSessionEncrypter{
		privateKey:   key,
		publicKey:    publicKey,
		mu:           sync.Mutex{},
		verifyTokens: map[*conn][]byte{},
	}, nil
}

func (encrypter *DefaultSessionEncrypter) PublicKey() []byte {
	encrypter.mu.Lock()
	defer encrypter.mu.Unlock()
	return encrypter.publicKey
}

func (encrypter *DefaultSessionEncrypter) GenerateVerifyToken(r *Request) ([]byte, error) {
	encrypter.mu.Lock()
	defer encrypter.mu.Unlock()

	verifyToken := make([]byte, verifyTokenLength)
	if _, err := rand.Read(verifyToken); err != nil {
		return nil, err
	}

	encrypter.verifyTokens[r.conn] = verifyToken
	return verifyToken, nil
}

func (encrypter *DefaultSessionEncrypter) DecryptAndVerifySharedSecret(r *Request, sharedSecret, verifyToken []byte) ([]byte, error) {
	encrypter.mu.Lock()
	defer encrypter.mu.Unlock()

	token, ok := encrypter.verifyTokens[r.conn]
	if !ok {
		return nil, errors.New("no verify token registered")
	}
	delete(encrypter.verifyTokens, r.conn)

	verifyToken, err := encrypter.privateKey.Decrypt(rand.Reader, verifyToken, nil)
	if err != nil {
		return nil, err
	}

	if !bytes.Equal(verifyToken, token) {
		return nil, errors.New("verify token did not match")
	}

	return encrypter.privateKey.Decrypt(rand.Reader, sharedSecret, nil)
}
