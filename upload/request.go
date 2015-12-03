package upload

import "gopkg.in/validator.v2"

var fileTypes = []string{
	"",
	"image",
}

// AssignRequest encompasses the fields optionally used to validate an upload
// before passing it onto seaweed. Current this only contains type and size
// but could later contain min image resolution, song duration, etc
type AssignRequest struct {
	// Currently only a FileType of "image" is supported
	FileType string `json:"type" mapstructure:"type" validate:"validType"`

	// The maximum number of bytes that the uploaded file can be
	MaxSize int64 `json:"maxSize" mapstructure:"maxSize" validate:"min=0"`

	// Replication is not used in dank and is just forwarded onto seaweedfs
	Replication string `json:"-" mapstructure:"replication"`
}

// compressedAssignRequest is just a compressed version of the AssignRequest
// that is used in the signature in order to make it smaller
type compressedAssignRequest struct {
	FileTypeIndex int   `msgpack:"i"`
	MaxSize       int64 `msgpack:"s"`
}

func init() {
	validator.SetValidationFunc("validType", validateType)
}

func stringTypeToIndex(t string) int {
	for i, v := range fileTypes {
		if v == t {
			return i
		}
	}
	return -1
}

func validateType(v interface{}, _ string) error {
	str, ok := v.(string)
	if !ok {
		return validator.ErrUnsupported
	}
	if str != "" && stringTypeToIndex(str) == -1 {
		return validator.ErrInvalid
	}
	return nil
}

// compress turns a AssignRequest into a compressedAssignRequest
func (r AssignRequest) compress() *compressedAssignRequest {
	return &compressedAssignRequest{
		FileTypeIndex: stringTypeToIndex(r.FileType),
		MaxSize:       r.MaxSize,
	}
}

// decompress turns a compressedAssignRequest into a decompress
func (r compressedAssignRequest) decompress() *AssignRequest {
	t := ""
	if r.FileTypeIndex > 0 && r.FileTypeIndex < len(fileTypes) {
		t = fileTypes[r.FileTypeIndex]
	}
	return &AssignRequest{
		FileType: t,
		MaxSize:  r.MaxSize,
	}
}
