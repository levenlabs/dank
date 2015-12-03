package upload

import (
	. "testing"

	"encoding/base64"
	"github.com/levenlabs/dank/seaweed"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"time"
)

func TestEncodeDecode(t *T) {
	r := &AssignRequest{
		FileType:   "image",
		MaxSizeStr: "1024",
		TTL:        "2m",
		//note: SigExpires is not transferred
	}
	fid := base64.URLEncoding.EncodeToString([]byte("hello"))
	f := fid + ".jpg"
	ar, err := seaweed.NewResult("localhost:8080", f)
	require.Nil(t, err)

	str, err := encode(r, ar)
	require.Nil(t, err)

	r2, ar2, err := decode(str, f)
	require.Nil(t, err)

	assert.EqualValues(t, r, r2)
	assert.EqualValues(t, ar, ar2)
}

func TestExpires(t *T) {
	fid := base64.URLEncoding.EncodeToString([]byte("hello"))
	f := fid + ".jpg"
	ar, err := seaweed.NewResult("localhost:8080", f)
	require.Nil(t, err)

	r := &AssignRequest{
		SigExpiresStr: "1",
	}
	str, err := encode(r, ar)
	require.Nil(t, err)

	_, _, err = decode(str, f)
	require.Nil(t, err)

	time.Sleep(2 * time.Second)

	_, _, err = decode(str, f)
	require.NotNil(t, err)
}
