package crypto

import (
	"crypto/rand"
	"encoding/binary"
	"io"

	"golang.org/x/crypto/nacl/box"
)

const (
	maxMessageSize = 65536
	minMessageSize = 24
)

func EncryptMessage(content []byte, recipientPublicKey *[32]byte, senderPrivateKey *[32]byte) ([]byte, error) {
	if len(content) > maxMessageSize {
		return nil, ErrMessageTooLarge
	}

	var nonce [24]byte
	if _, err := io.ReadFull(rand.Reader, nonce[:]); err != nil {
		return nil, err
	}

	encrypted := box.Seal(nonce[:], content, &nonce, recipientPublicKey, senderPrivateKey)
	return encrypted, nil
}

func DecryptMessage(encrypted []byte, senderPublicKey *[32]byte, recipientPrivateKey *[32]byte) ([]byte, error) {
	if len(encrypted) < minMessageSize {
		return nil, ErrInvalidMessage
	}

	var nonce [24]byte
	copy(nonce[:], encrypted[:24])
	ciphertext := encrypted[24:]

	decrypted, ok := box.Open(nil, ciphertext, &nonce, senderPublicKey, recipientPrivateKey)
	if !ok {
		return nil, ErrDecryptionFailed
	}

	return decrypted, nil
}

func PackEncryptedMessages(messages [][]byte) []byte {
	if len(messages) == 0 {
		return nil
	}
	var result []byte
	for _, msg := range messages {
		sizeBuf := make([]byte, 4)
		binary.BigEndian.PutUint32(sizeBuf, uint32(len(msg)))
		result = append(result, sizeBuf...)
		result = append(result, msg...)
	}
	return result
}

func UnpackEncryptedMessages(data []byte) ([][]byte, error) {
	var messages [][]byte
	for len(data) >= 4 {
		size := binary.BigEndian.Uint32(data[:4])
		data = data[4:]

		if size > maxMessageSize || len(data) < int(size) || int(size) < minMessageSize {
			return nil, ErrInvalidMessage
		}

		messages = append(messages, data[:size])
		data = data[size:]
	}
	return messages, nil
}

type EncryptionError string

func (e EncryptionError) Error() string {
	return string(e)
}

const (
	ErrMessageTooLarge  = EncryptionError("message too large")
	ErrInvalidMessage   = EncryptionError("invalid message format")
	ErrDecryptionFailed = EncryptionError("decryption failed")
)

