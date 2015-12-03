package upload

import (
	"github.com/levenlabs/go-llog"
	"github.com/levenlabs/dank/config"
	"github.com/levenlabs/dank/seaweed"
	"crypto/aes"
"crypto/cipher"
"crypto/rand"
	"encoding/base64"
	"strings"
	"fmt"
	"gopkg.in/vmihailenco/msgpack.v2"
	"hash/crc32"
)

type Signature struct {
	Req *CompressedAssignRequest `msgpack:"r"`
	SeaweedURL string `msgpack:"u"`
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

func encode(r *AssignRequest, ar *seaweed.AssignResult) (string, error) {
	g, err := gcm()
	if err != nil {
		return "", err
	}
	nonce := make([]byte, g.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", err
	}

	sig := &Signature{
		Req: r.Compress(),
		SeaweedURL: ar.URL,
		CRC: crc32.ChecksumIEEE([]byte(ar.Filename())),
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

	sig := &Signature{}
	err = msgpack.Unmarshal(v, sig)
	if err != nil {
		return nil, nil, err
	}

	ar, err := seaweed.NewResult(sig.SeaweedURL, f)
	if err != nil || crc32.ChecksumIEEE([]byte(ar.Filename())) != sig.CRC {
		return nil, nil, fmt.Errorf("unauthorized filname sent")
	}

	return sig.Req.Decompress(), ar, nil
}
