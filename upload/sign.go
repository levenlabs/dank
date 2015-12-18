package upload

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"github.com/levenlabs/dank/config"
	"github.com/levenlabs/dank/seaweed"
	"github.com/levenlabs/dank/types"
	"github.com/levenlabs/go-llog"
	"gopkg.in/vmihailenco/msgpack.v2"
	"hash/crc32"
	"strings"
	"time"
)

//todo: RawURLEncoding
var encoder = base64.URLEncoding

type signature struct {
	Req        *compressedAssignRequest `msgpack:"r"`
	SeaweedURL string                   `msgpack:"u"`

	// the CRC is of the filename of the AssignRequest
	CRC uint32 `msgpack:"c"`

	// Expires represents the unix time that this expires
	Expires int64 `msgpack:"e"`
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
func encode(r *types.AssignRequest, ar *seaweed.AssignResult) (string, error) {
	kv := llog.KV{
		"filename": ar.Filename(),
	}
	g, err := gcm()
	if err != nil {
		kv["error"] = err
		llog.Error("error creating gcm", kv)
		return "", err
	}
	nonce := make([]byte, g.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		kv["error"] = err
		llog.Error("error filling nonce", kv)
		return "", err
	}

	sig := &signature{
		Req:        compressRequest(r),
		SeaweedURL: ar.Host(),
		CRC:        crc32.ChecksumIEEE([]byte(ar.Filename())),
		Expires:    r.Expires(),
	}
	b, err := msgpack.Marshal(sig)
	if err != nil {
		kv["error"] = err
		llog.Error("error marshaling msgpack", kv)
		return "", err
	}
	res := strings.Join([]string{
		"1",
		encoder.EncodeToString(nonce),
		encoder.EncodeToString(g.Seal(nil, nonce, b, nil)),
	}, "$")
	return res, nil
}

// decode takes the encrypted string signature from encode and the filename
// and validates that the filename matches the one originally sent to encode.
// It returns the original AssignRequest and a new seaweed.AssignResult that can
// be used to upload the file
func decode(s string, f string) (*types.AssignRequest, *seaweed.AssignResult, error) {
	kv := llog.KV{
		"string": s,
	}
	parts := strings.Split(s, "$")
	if len(parts) != 3 || parts[0] != "1" {
		kv["len"] = len(parts)
		llog.Debug("number of parts was invalid", kv)
		return nil, nil, errors.New("invalid signature")
	}
	nonce, err := encoder.DecodeString(parts[1])
	if err != nil {
		kv["error"] = err
		llog.Debug("error base64 decoding signature part 1", kv)
		return nil, nil, err
	}
	c, err := encoder.DecodeString(parts[2])
	if err != nil {
		kv["error"] = err
		llog.Debug("error base64 decoding signature part 2", kv)
		return nil, nil, err
	}
	g, err := gcm()
	if err != nil {
		kv["error"] = err
		llog.Error("error creating gcm", kv)
		return nil, nil, err
	}
	v, err := g.Open(nil, nonce, c, nil)
	if err != nil {
		return nil, nil, err
	}

	sig := &signature{}
	err = msgpack.Unmarshal(v, sig)
	if err != nil {
		kv["error"] = err
		kv["string"] = v
		llog.Error("error unmarshaling msgpack", kv)
		return nil, nil, err
	}

	if sig.Expires > 0 && time.Now().UTC().Unix() > sig.Expires {
		kv["expires"] = sig.Expires
		llog.Debug("signature expired", kv)
		return nil, nil, errors.New("signature expired")
	}

	ar, err := seaweed.NewAssignResult(sig.SeaweedURL, f)
	if err != nil || crc32.ChecksumIEEE([]byte(ar.Filename())) != sig.CRC {
		kv["error"] = err
		kv["crc"] = sig.CRC
		kv["filename"] = f
		llog.Debug("error with checksum or filename", kv)
		return nil, nil, fmt.Errorf("unauthorized filename sent")
	}

	return sig.Req.decompress(), ar, nil
}
