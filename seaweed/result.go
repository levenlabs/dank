package seaweed

import (
	"encoding/base64"
	"strings"
)

//todo: RawURLEncoding
var encoder = base64.URLEncoding

// AssignResult holds the result of the assign call to seaweed. It exposes
// two methods to get the Filename and the URL
type AssignResult struct {
	fid string
	url string
}

// Returns the filename useful for uploading. It's base64-encoded to ensure url
// acceptance and to hide any seaweed formatting
func (r *AssignResult) Filename() string {
	return encoder.EncodeToString([]byte(r.fid))
}

// Returns the fid of the file which can be used for uploading to seaweed
// manually
func (r *AssignResult) FID() string {
	return r.fid
}

// Returns the host:port of the seaweed volume that contains this file. This is
// only exposed for hiding this value in the signature
func (r *AssignResult) Host() string {
	return r.url
}

// Returns the full URL that contains this file.
func (r *AssignResult) URL() string {
	return "http://" + r.url + "/" + r.fid
}

// decodes the filename and strips off any file extension and un-base64's the
// filename to get the fid
func decodeFilename(f string) (string, error) {
	parts := strings.Split(f, ".")
	fid, err := encoder.DecodeString(parts[0])
	if err != nil {
		return "", err
	}
	return string(fid), nil
}

// NewAssignResult returns a AssignResult from a url and filename. This is used when
// a signature is decoded
func NewAssignResult(u, filename string) (*AssignResult, error) {
	fid, err := decodeFilename(filename)
	if err != nil {
		return nil, err
	}
	return &AssignResult{
		fid: fid,
		url: u,
	}, nil
}

// NewRawAssignResult returns a AssignResult from a url and fid. This is used
// when decoding rawAssignResults
func NewRawAssignResult(u, fid string) *AssignResult {
	return &AssignResult{
		fid: fid,
		url: u,
	}
}
