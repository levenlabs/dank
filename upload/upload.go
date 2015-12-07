package upload

import (
	"bytes"
	dhttp "github.com/levenlabs/dank/http"
	"github.com/levenlabs/dank/seaweed"
	"github.com/levenlabs/go-llog"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"io/ioutil"
	"net/http"
)

// Assignment holds a signature and a filename which are needed to upload a
// file and validate it. Since mapstructure doesn't support embedded structs
// we have to copy these to main.go's uploadArgs
type Assignment struct {
	Signature string `json:"sig" mapstructure:"sig"  validate:"nonzero"`
	Filename  string `json:"filename"  mapstructure:"filename" validate:"nonzero"`
}

// Assign takes an AssignRequest and returns an Assignment that can be used to
// upload a file later
func Assign(r *AssignRequest) (*Assignment, error) {
	ar, err := seaweed.Assign(r.Replication, r.TTL)
	if err != nil {
		return nil, err
	}

	sig, err := encode(r, ar)
	if err != nil {
		return nil, err
	}

	llog.Info("created signature for file", llog.KV{
		"filename": ar.Filename(),
		"url":      ar.URL(),
		"sig":      sig,
		"maxSize":  r.MaxSize(),
		"fileType": r.FileType,
		"expires":  r.SigExpiresStr,
	})

	a := &Assignment{
		Signature: sig,
		Filename:  ar.Filename(),
	}
	return a, nil
}

// Upload takes an Assignment and a body and verifies that the body abides to
// the original AssignRequest and then uploads the body to seaweed. len should
// indicate the length of the body. This can be http.Request's ContentLength
//
// If a MaxSize was specified in the original AssignRequest, then the body
// io.Reader is only read until the MaxSize
func Upload(a *Assignment, body io.Reader, blen int64) error {
	r, ar, err := decode(a.Signature, a.Filename)
	if err != nil {
		llog.Info("error running decode in upload", llog.KV{
			"error":    err,
			"filename": a.Filename,
			"sig":      a.Signature,
		})
		return dhttp.NewError(http.StatusBadRequest, "invalid signature or filename")
	}

	maxSize := r.MaxSize()
	kv := llog.KV{
		"filename": ar.Filename(),
		"len":      blen,
		"fileType": r.FileType,
		"maxSize":  maxSize,
	}
	if r.TTL != "" {
		kv["ttl"] = r.TTL
	}

	llog.Debug("checking filesize", kv)
	if maxSize > 0 {
		if blen > maxSize {
			return dhttp.NewError(http.StatusRequestEntityTooLarge, "request is larger than %d bytes", maxSize)
		}
		body = io.LimitReader(body, maxSize)
	}

	ok := true
	switch r.FileType {
	case "image":
		var b []byte
		b, err = ioutil.ReadAll(body)
		if err != nil {
			kv["error"] = err
			llog.Info("error running ioutil.ReadAll", kv)
			return dhttp.NewError(http.StatusBadRequest, "invalid body uploaded")
		}
		_, _, err := image.Decode(bytes.NewBuffer(b))
		if err != nil {
			kv["error"] = err
			if len(b) >= 3 {
				kv["bytes"] = b[0:3]
			}
			llog.Info("error running image.Decode", kv)
			ok = false
		} else {
			body = ioutil.NopCloser(bytes.NewBuffer(b))
		}
	}

	if ok {
		llog.Info("uploading file to seaweed", kv)
	} else {
		return dhttp.NewError(http.StatusBadRequest,
			"uploaded file could not be validated as %s", r.FileType)
	}

	return seaweed.Upload(ar, body, r.TTL)
}

// Verify takes an assignment and validates the filename to the signature
func Verify(a *Assignment) error {
	_, _, err := decode(a.Signature, a.Filename)
	if err != nil {
		llog.Info("error running decode in upload", llog.KV{
			"error":    err,
			"filename": a.Filename,
			"sig":      a.Signature,
		})
		return dhttp.NewError(http.StatusBadRequest, "invalid signature or filename")
	}
	return nil
}
