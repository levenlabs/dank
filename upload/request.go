package upload

import "gopkg.in/validator.v2"

var fileTypes = []string{
	"",
	"image",
}

type AssignRequest struct {
	FileType string `json:"type" mapstructure:"type" validate:"validType"`
	MaxSize  int `json:"maxSize" mapstructure:"maxSize" validate:"min=0"`
}

type CompressedAssignRequest struct {
	FileTypeIndex int `msgpack:"i"`
	MaxSize       int `msgpack:"s"`
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

func (r AssignRequest) Compress() *CompressedAssignRequest {
	return &CompressedAssignRequest{
		FileTypeIndex: stringTypeToIndex(r.FileType),
		MaxSize: r.MaxSize,
	}
}

func (r CompressedAssignRequest) Decompress() *AssignRequest {
	t := ""
	if r.FileTypeIndex > 0 && r.FileTypeIndex < len(fileTypes) {
		t = fileTypes[r.FileTypeIndex]
	}
	return &AssignRequest{
		FileType: t,
		MaxSize: r.MaxSize,
	}
}
