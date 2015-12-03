package upload

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"github.com/levenlabs/dank/config"
	"github.com/levenlabs/dank/seaweed"
	"github.com/levenlabs/go-llog"
	"gopkg.in/vmihailenco/msgpack.v2"
	"hash/crc32"
	"strings"
)

type signature struct {
	Req        *compressedAssignRequest `msgpack:"r"`
	SeaweedURL string                   `msgpack:"u"`
	// the CRC is of the filename of the AssignRequest
	CRC uint32 `msgpack:"c"`
}

func init() {
	if config.Secret == "" {
		llog.Fatal("--secret is required")
	}
}

func gcm() (cipher.AEAD, error) {
	key := []byte(config.Secret)
	b, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(b)
}

// encode returns an encrypted string signature for the given AssignRequest and
// seaweed.AssignResult. It crc's the filename from the result and uses a gcm
// cipher to encrypt the signature struct
func encode(r *AssignRequest, ar *seaweed.AssignResult) (string, error) {
	g, err := gcm()
	if err != nil {
		return "", err
	}
	nonce := make([]byte, g.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", err
	}

	sig := &signature{
		Req:        r.compress(),
		SeaweedURL: ar.URL(),
		CRC:        crc32.ChecksumIEEE([]byte(ar.Filename())),
	}
	b, err := msgpack.Marshal(sig)
	if err != nil {
		return "", err
	}
	res := strings.Join([]string{
		"1",
		base64.StdEncoding.EncodeToString(nonce),
		base64.StdEncoding.EncodeToString(g.Seal(nil, nonce, b, nil)),
	}, "$")
	return res, nil
}

// decode takes the encrypted string signature from encode and the filename
// and validates that the filename matches the one originally sent to encode.
// It returns the original AssignRequest and a new seaweed.AssignResult that can
// be used to upload the file
func decode(s string, f string) (*AssignRequest, *seaweed.AssignResult, error) {
	parts := strings.Split(s, "$")
	if len(parts) != 3 || parts[0] != "1" {
		return nil, nil, fmt.Errorf("invalid signature")
	}
	nonce, err := base64.StdEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, nil, fmt.Errorf("invalid signature")
	}
	c, err := base64.StdEncoding.DecodeString(parts[2])
	if err != nil {
		return nil, nil, fmt.Errorf("invalid signature")
	}
	g, err := gcm()
	if err != nil {
		return nil, nil, err
	}
	v, err := g.Open(nil, nonce, c, nil)
	if err != nil {
		return nil, nil, err
	}

	sig := &signature{}
	err = msgpack.Unmarshal(v, sig)
	if err != nil {
		return nil, nil, err
	}

	ar, err := seaweed.NewResult(sig.SeaweedURL, f)
	if err != nil || crc32.ChecksumIEEE([]byte(ar.Filename())) != sig.CRC {
		return nil, nil, fmt.Errorf("unauthorized filename sent")
	}

	return sig.Req.decompress(), ar, nil
}
