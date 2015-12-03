package upload

import (
	. "testing"

	"encoding/base64"
	"github.com/levenlabs/dank/seaweed"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestVerify(t *T) {
	r := &AssignRequest{"image", 1024, "", ""}
	fid := base64.URLEncoding.EncodeToString([]byte("hello"))
	f := fid + ".jpg"
	ar, err := seaweed.NewResult("localhost:8080", f)
	require.Nil(t, err)

	str, err := encode(r, ar)
	require.Nil(t, err)

	a := &Assignment{
		Signature: str,
		Filename:  f,
	}
	err = Verify(a)
	assert.Nil(t, err)
}
