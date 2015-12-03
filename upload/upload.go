package upload

import (
	"bytes"
	"fmt"
	"github.com/levenlabs/dank/seaweed"
	"image"
	"io"
	"io/ioutil"
)

// Assignment holds a signature and a filename which are needed to upload a
// file and validate it
type Assignment struct {
	Signature string `json:"sig" mapstructure:"sig"  validate:"nonzero"`
	Filename  string `json:"filename"  mapstructure:"filename" validate:"nonzero"`
}

// Assign takes an AssignRequest and returns an Assignment that can be used to
// upload a file later
func Assign(r *AssignRequest) (*Assignment, error) {
	ar, err := seaweed.Assign(r.Replication)
	if err != nil {
		return nil, err
	}

	sig, err := encode(r, ar)
	if err != nil {
		return nil, err
	}

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
func Upload(a *Assignment, body io.Reader, len int64) error {
	r, ar, err := decode(a.Signature, a.Filename)
	if err != nil {
		return err
	}
	if r.MaxSize > 0 {
		if len > r.MaxSize {
			return fmt.Errorf("request too large")
		}
		body = io.LimitReader(body, r.MaxSize)
	}

	switch r.FileType {
	case "image":
		var b []byte
		if b, err = ioutil.ReadAll(body); err != nil {
			_, _, err := image.Decode(bytes.NewBuffer(b))
			if err != nil {
				return fmt.Errorf("invalid filetype uploaded")
			}
		}
		body = ioutil.NopCloser(bytes.NewBuffer(b))
	}

	return seaweed.Upload(ar, body)
}

// Verify takes an assignment and validates the filename to the signature
func Verify(a *Assignment) error {
	_, _, err := decode(a.Signature, a.Filename)
	if err != nil {
		return err
	}
	return nil
}
