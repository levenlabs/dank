package types

import (
	"github.com/levenlabs/go-llog"
	"gopkg.in/validator.v2"
	"net/url"
	"strconv"
	"time"
)

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
	// This is a string value so mapstructure can handle it, use MaxSize() to get
	// the int64 value
	MaxSizeStr string `json:"max_size" mapstructure:"max_size" validate:"regexp=^[0-9]*$"`

	// Replication is not used in dank and is just forwarded onto seaweedfs
	Replication string `json:"replication" mapstructure:"replication"`

	// TTL is stored and sent to seaweedfs in the assign and upload steps
	TTL string `json:"ttl" mapstructure:"ttl"`

	// SigExpires sets the expires time on the generated signature to this
	// number of seconds. By default this value is 0 which means don't expire.
	// This is a string value so mapstructure can handle it, use Expires() to
	// get the unix timestamp when the expires is
	SigExpiresStr string `json:"sigExpires" mapstructure:"sig_expires" validate:"regexp=^[0-9]*$"`
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
		llog.Warn("fileType was unknown", llog.KV{
			"string": str,
		})
		return validator.ErrInvalid
	}
	return nil
}

// expires returns at what unix time a signature generated with this request
// expires or 0 if it never expires
func (r *AssignRequest) Expires() int64 {
	if r.SigExpiresStr == "" || r.SigExpiresStr == "0" {
		return 0
	}
	i, _ := strconv.ParseInt(r.SigExpiresStr, 10, 64)
	if i == 0 {
		return 0
	}
	return time.Now().Add(time.Duration(i) * time.Second).UTC().Unix()
}

func (r *AssignRequest) MaxSize() int64 {
	if r.MaxSizeStr == "" || r.MaxSizeStr == "0" {
		return 0
	}
	i, _ := strconv.ParseInt(r.MaxSizeStr, 10, 64)
	return i
}

func (r *AssignRequest) FileTypeID() int {
	return stringTypeToIndex(r.FileType)
}

func (r *AssignRequest) URLValues() url.Values {
	v := make(url.Values)
	if r.FileType != "" {
		v.Set("type", r.FileType)
	}
	if r.MaxSizeStr != "" {
		v.Set("max_size", r.MaxSizeStr)
	}
	if r.Replication != "" {
		v.Set("replication", r.Replication)
	}
	if r.TTL != "" {
		v.Set("ttl", r.TTL)
	}
	if r.SigExpiresStr != "" {
		v.Set("sig_expires", r.SigExpiresStr)
	}
	return v
}

func FileTypeFromID(i int) string {
	t := ""
	if i > 0 && i < len(fileTypes) {
		t = fileTypes[i]
	}
	return t
}
