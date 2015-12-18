package upload

import (
	"strconv"
	"github.com/levenlabs/dank/types"
)

// compressedAssignRequest is just a compressed version of the AssignRequest
// that is used in the signature in order to make it smaller
//
// since SigExpires is stored on the signature itself we don't need it here
type compressedAssignRequest struct {
	FileTypeIndex int    `msgpack:"i"`
	MaxSize       int64  `msgpack:"s"`
	TTL           string `msgpack:"t"`
}

// compress turns a AssignRequest into a compressedAssignRequest
func compressRequest(r *types.AssignRequest) *compressedAssignRequest {
	return &compressedAssignRequest{
		FileTypeIndex: r.FileTypeID(),
		MaxSize:       r.MaxSize(),
		TTL:           r.TTL,
	}
}

// decompress turns a compressedAssignRequest into a decompress
func (r compressedAssignRequest) decompress() *types.AssignRequest {
	return &types.AssignRequest{
		FileType:   types.FileTypeFromID(r.FileTypeIndex),
		MaxSizeStr: strconv.FormatInt(r.MaxSize, 10),
		TTL:        r.TTL,
	}
}
