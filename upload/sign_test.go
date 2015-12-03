package upload

import (
	. "testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/levenlabs/dank/seaweed"
"fmt"
	"encoding/base64"
)

func TestEncodeDecode(t *T) {
	r := &AssignRequest{"image", 1024}
	fid := base64.URLEncoding.EncodeToString([]byte("hello"))
	f := fid + ".jpg"
	ar, err := seaweed.NewResult("localhost:8080", f)
	fmt.Printf("%+v", err)
	require.Nil(t, err)

	str, err := encode(r, ar)
	require.Nil(t, err)

	r2, ar2, err := decode(str, f)
	require.Nil(t, err)

	assert.EqualValues(t, r, r2)
	assert.EqualValues(t, ar, ar2)
}
