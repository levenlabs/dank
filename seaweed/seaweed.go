package seaweed

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/levenlabs/dank/config"
	"github.com/levenlabs/go-llog"
	"io"
	"math/rand"
	"net/http"
	"strings"
)

// AssignResult holds the result of the assign call to seaweed. It exposes
// two methods to get the Filename and the URL
type AssignResult struct {
	fid string
	url string
}

// rawAssignResult is only used to Unmarshal into and then an AssignResult is
// made to publicly return
type rawAssignResult struct {
	FID string `json:"fid"`
	URL string `json:"url"`
}

type lookupResult struct {
	Locations []location `json:"locations"`
}

type location struct {
	URL string `json:"url"`
}

var assignURL string
var lookupURL string

func init() {
	if config.SeaweedAddr == "" {
		llog.Fatal("--seaweed-addr is required")
	}
	//todo: wtf?
	rand.Seed(rand.Int63())

	assignURL = "http://" + config.SeaweedAddr + "/dir/assign"
	lookupURL = "http://" + config.SeaweedAddr + "/dir/lookup?volumeId="
}

// Returns the filename useful for uploading. It's base64-encoded to ensure url
// acceptance and to hide any seaweed formatting
func (r *AssignResult) Filename() string {
	return base64.URLEncoding.EncodeToString([]byte(r.fid))
}

// Returns the host:port of the seaweed volume that contains this file. This is
// only exposed for hiding this value in the signature
func (r *AssignResult) URL() string {
	return r.url
}

// assignResult returns a public AssignResult from a rawAssignResult
func (r *rawAssignResult) assignResult() *AssignResult {
	return &AssignResult{
		fid: r.FID,
		url: r.URL,
	}
}

// decodes the filename and strips off any file extension and un-base64's the
// filename to get the fid
func decodeFilename(f string) (string, error) {
	parts := strings.Split(f, ".")
	fid, err := base64.URLEncoding.DecodeString(parts[0])
	if err != nil {
		return "", err
	}
	return string(fid), nil
}

// NewResult returns a AssignResult from a url and filename. This is used when
// a signature is decoded
func NewResult(url, filename string) (*AssignResult, error) {
	fid, err := decodeFilename(filename)
	if err != nil {
		return nil, err
	}
	return &AssignResult{
		fid: fid,
		url: url,
	}, nil
}

// Assign makes an assign call to seaweed to get a filename that can be uploaded
// to and returns an AssignResult. Optionally replication can be sent to
// guarantee the replication of the file. See the seaweedfs docs for values.
func Assign(replication string) (*AssignResult, error) {
	url := assignURL
	if replication != "" {
		url += "?replication=" + replication
	}
	resp, err := http.Post(url, "", nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("unexpected non-200 post status: %s", resp.Status)
	}
	r := &rawAssignResult{}
	err = json.NewDecoder(resp.Body).Decode(r)
	if err != nil {
		return nil, err
	}
	return r.assignResult(), nil
}

// Upload takes an existing AssignResult call that has already been validated
// and a io.Reader body. It uploads the body to the sent seaweed volume and
// fid
func Upload(r *AssignResult, body io.Reader) error {
	req, err := http.NewRequest("PUT", "http://"+r.url+"/"+r.fid, body)
	if err != nil {
		return err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return fmt.Errorf("unexpected non-200 put status: %s", resp.Status)
	}
	return nil
}

// Get takes the given filename, gets the file from seaweed, and writes it to
// the passed io.Writer
func Get(filename string, w io.Writer) error {
	fid, err := decodeFilename(filename)
	if err != nil {
		return err
	}
	//fid's format is volumeId,somestuff
	parts := strings.Split(fid, ",")
	resp, err := http.Get(lookupURL + parts[0])
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	r := &lookupResult{}
	err = json.NewDecoder(resp.Body).Decode(r)
	if err != nil {
		return err
	}
	if len(r.Locations) == 0 {
		return fmt.Errorf("not found")
	}
	i := rand.Intn(len(r.Locations))
	url := r.Locations[i].URL

	resp, err = http.Get("http://" + url + "/" + fid)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	_, err = io.Copy(w, resp.Body)
	return err
}
