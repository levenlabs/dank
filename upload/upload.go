package upload

import (
	"github.com/levenlabs/dank/seaweed"
	"bytes"
	"fmt"
	"image"
)

type Assignment struct {
	Signature string `json:"sig" mapstructure:"sig"  validate:"nonzero"`
	Filename string `json:"filename"  mapstructure:"filname" validate:"nonzero"`
}

func Assign(r *AssignRequest) (*Assignment, error)  {
	ar, err := seaweed.Assign()
	if err != nil {
		return nil, err
	}

	sig, err := encode(r, ar)
	if err != nil {
		return nil, err
	}

	a := &Assignment{
		Signature: sig,
		Filename: ar.Filename(),
	}
	return a, nil
}

func Upload(a *Assignment, body []byte) error {
	r, ar, err := decode(a.Signature, a.Filename)
	if err != nil {
		return err
	}
	if len(body) > r.MaxSize {
		return fmt.Errorf("request too large")
	}

	switch(r.FileType) {
	case "image":
		_, _, err := image.Decode(bytes.NewBuffer(body))
		if err == nil {
			return fmt.Errorf("invalid filetype uploaded")
		}
	}

	buf := bytes.NewBuffer(body)
	return seaweed.Upload(ar, buf)
}

func Verify(a *Assignment) error {
	_, _, err := decode(a.Signature, a.Filename)
	if err != nil {
		return err
	}
	return nil
}
