package seaweed

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/levenlabs/dank/config"
	"github.com/levenlabs/go-llog"
	"io"
	"io/ioutil"
	"math/rand"
	"mime/multipart"
	"net/http"
	"net/url"
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

//todo: RawURLEncoding
var encoder = base64.URLEncoding
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
	return encoder.EncodeToString([]byte(r.fid))
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
	fid, err := encoder.DecodeString(parts[0])
	if err != nil {
		return "", err
	}
	return string(fid), nil
}

// NewResult returns a AssignResult from a url and filename. This is used when
// a signature is decoded
func NewResult(u, filename string) (*AssignResult, error) {
	fid, err := decodeFilename(filename)
	if err != nil {
		return nil, err
	}
	return &AssignResult{
		fid: fid,
		url: u,
	}, nil
}

// Assign makes an assign call to seaweed to get a filename that can be uploaded
// to and returns an AssignResult. Optionally replication can be sent to
// guarantee the replication of the file and ttl can be sent to expire the file
// after a specific amount of time. See the seaweedfs docs.
func Assign(replication, ttl string) (*AssignResult, error) {
	u, err := url.Parse(assignURL)
	if err != nil {
		return nil, err
	}
	q := u.Query()
	if replication != "" {
		q.Set("replication", replication)
	}
	if ttl != "" {
		q.Set("ttl", ttl)
	}
	u.RawQuery = q.Encode()
	uStr := u.String()

	kv := llog.KV{
		"url": uStr,
	}
	llog.Debug("making seaweed GET request", kv)

	resp, err := http.Get(uStr)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		//see if there's a body
		if body, err := ioutil.ReadAll(resp.Body); err == nil {
			return nil, fmt.Errorf("unexpected seaweed status (%s): %s", resp.Status, string(body))
		}
		return nil, fmt.Errorf("unexpected seaweed status (%s)", resp.Status)
	}

	r := &rawAssignResult{}
	err = json.NewDecoder(resp.Body).Decode(r)
	if err != nil {
		kv["error"] = err
		llog.Error("error decoding assign response from seaweed", kv)
		return nil, err
	}
	return r.assignResult(), nil
}

// Upload takes an existing AssignResult call that has already been validated
// and a io.Reader body. It uploads the body to the sent seaweed volume and
// fid. Optionally it passes along a ttl to seaweed.
func Upload(r *AssignResult, body io.Reader, ttl string) error {
	u, err := url.Parse("http://" + r.url + "/" + r.fid)
	if err != nil {
		return err
	}
	q := u.Query()
	if ttl != "" {
		q.Set("ttl", ttl)
	}
	u.RawQuery = q.Encode()
	uStr := u.String()
	llog.Debug("making seaweed PUT request", llog.KV{
		"url": uStr,
	})

	// we HAVE to upload a form the file in file
	newBody := &bytes.Buffer{}
	mpw := multipart.NewWriter(newBody)
	part, err := mpw.CreateFormFile("file", r.Filename())
	if err != nil {
		return err
	}
	_, err = io.Copy(part, body)
	if err != nil {
		return err
	}
	err = mpw.Close()
	if err != nil {
		return err
	}

	req, err := http.NewRequest("PUT", uStr, newBody)
	if err != nil {
		return err
	}
	req.Header.Add("Content-Type", mpw.FormDataContentType())
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 201 {
		//see if there's a body
		if body, err := ioutil.ReadAll(resp.Body); err == nil {
			return fmt.Errorf("unexpected seaweed status (%s): %s", resp.Status, string(body))
		}
		return fmt.Errorf("unexpected seaweed status (%s)", resp.Status)
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
	uStr := lookupURL + parts[0]

	kv := llog.KV{
		"url": uStr,
	}
	llog.Debug("making seaweed GET request", kv)

	resp, err := http.Get(uStr)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	r := &lookupResult{}
	err = json.NewDecoder(resp.Body).Decode(r)
	if err != nil {
		kv["error"] = err
		llog.Error("error decoding get response from seaweed", kv)
		return err
	}
	if len(r.Locations) == 0 {
		return fmt.Errorf("not found")
	}
	i := rand.Intn(len(r.Locations))
	u := r.Locations[i].URL
	uStr = "http://" + u + "/" + fid

	kv["url"] = uStr
	llog.Debug("making seaweed GET request", kv)

	resp, err = http.Get(uStr)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	_, err = io.Copy(w, resp.Body)
	return err
}
