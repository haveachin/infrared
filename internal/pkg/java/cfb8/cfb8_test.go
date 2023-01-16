package cfb8

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"testing"
)

func initCipherBlock(secret []byte) (cipher.Block, error) {
	block, err := aes.NewCipher(secret)
	if err != nil {
		return nil, err
	}

	return block, nil
}

func TestCfb8_XORKeyStream_Encrypt(t *testing.T) {
	secret := []byte("qwdhyte62kjneThg")
	block, err := initCipherBlock(secret)
	if err != nil {
		t.Error(err)
	}

	encrypter := newCFB8(block, secret, false)
	message := "Hello"
	should := []byte{68, 159, 26, 206, 126}
	is := make([]byte, len(should))

	encrypter.XORKeyStream(is, []byte(message))

	if bytes.Compare(is, should) != 0 {
		t.Fail()
	}
}

func TestCfb8_XORKeyStream_Decrypt(t *testing.T) {
	secret := []byte("qwdhyte62kjneThg")
	block, err := initCipherBlock(secret)
	if err != nil {
		t.Error(err)
	}

	decrypter := newCFB8(block, secret, true)
	message := []byte{68, 159, 26, 206, 126}
	should := "Hello"
	is := make([]byte, len(should))

	decrypter.XORKeyStream(is, message)

	if bytes.Compare(is, []byte(should)) != 0 {
		t.Fail()
	}
}

func initRandomCipherBlock() ([]byte, cipher.Block, error) {
	secret := make([]byte, 16)
	if _, err := rand.Read(secret); err != nil {
		return nil, nil, err
	}

	block, err := initCipherBlock(secret)
	return secret, block, err
}

func BenchmarkEncrypt10000Bytes(b *testing.B) {
	secret, block, err := initRandomCipherBlock()
	if err != nil {
		b.Error(err)
	}

	c := NewEncrypter(block, secret)
	dst := make([]byte, 10000)
	src := make([]byte, 10000)
	if _, err := rand.Read(src); err != nil {
		b.Error(err)
	}

	for n := 0; n < b.N; n++ {
		c.XORKeyStream(dst, src)
	}
}

func BenchmarkDecrypt10000Bytes(b *testing.B) {
	secret, block, err := initRandomCipherBlock()
	if err != nil {
		b.Error(err)
	}

	c := NewDecrypter(block, secret)
	dst := make([]byte, 10000)
	src := make([]byte, 10000)
	if _, err := rand.Read(src); err != nil {
		b.Error(err)
	}

	for n := 0; n < b.N; n++ {
		c.XORKeyStream(dst, src)
	}
}
