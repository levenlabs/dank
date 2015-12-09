package seaweed

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"github.com/levenlabs/dank/config"
	dhttp "github.com/levenlabs/dank/http"
	"github.com/levenlabs/go-llog"
	"github.com/levenlabs/go-srvclient"
	"io"
	"io/ioutil"
	"math/rand"
	"mime/multipart"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"
	"time"
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

func init() {
	if config.SeaweedAddr == "" {
		llog.Fatal("--seaweed-addr is required")
	}
	rand.Seed(time.Now().UnixNano())
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

// intInList determines if the int i is in the list l
func intInList(i int, l []int) bool {
	for _, v := range l {
		if v == i {
			return true
		}
	}
	return false
}

func doReq(req *http.Request, kv llog.KV, expectedCodes ...int) (*http.Response, int, error) {
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		kv["error"] = err
		llog.Warn("error making seaweed http request", kv)
		return nil, 0, err
	}
	if code, err := handleResp(resp, kv, expectedCodes...); err != nil {
		//return nil here since the handleResp closed the body already
		return nil, code, err
	}
	return resp, resp.StatusCode, nil
}

func handleResp(resp *http.Response, kv llog.KV, expectedCodes ...int) (int, error) {
	if !intInList(resp.StatusCode, expectedCodes) {
		defer resp.Body.Close()
		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			kv["body"] = body
		}
		kv["status"] = resp.Status
		llog.Warn("invalid seaweed status", kv)
		return resp.StatusCode, errors.New("unexpected seaweed status")
	}
	return resp.StatusCode, nil
}

// Assign makes an assign call to seaweed to get a filename that can be uploaded
// to and returns an AssignResult. Optionally replication can be sent to
// guarantee the replication of the file and ttl can be sent to expire the file
// after a specific amount of time. See the seaweedfs docs.
func Assign(replication, ttl string) (*AssignResult, error) {
	addr := srvclient.MaybeSRV(config.SeaweedAddr)
	uStr := "http://" + addr + "/dir/assign"
	u, err := url.Parse(uStr)
	if err != nil {
		llog.Error("error building seaweed url", llog.KV{
			"addr": addr,
		})
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
	uStr = u.String()

	kv := llog.KV{
		"url": uStr,
	}
	llog.Debug("making seaweed GET request", kv)

	resp, err := http.Get(uStr)
	if err != nil {
		kv["error"] = err
		llog.Warn("error making seaweed http request", kv)
		return nil, err
	}
	if _, err = handleResp(resp, kv, http.StatusOK); err != nil {
		return nil, err
	}
	defer resp.Body.Close()

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
func Upload(r *AssignResult, body io.Reader, ct string, urlParams map[string]string) error {
	u, err := url.Parse("http://" + r.url + "/" + r.fid)
	if err != nil {
		llog.Error("error building seaweed url", llog.KV{
			"url": r.url,
			"fid": r.fid,
		})
		return err
	}
	if len(urlParams) > 0 {
		q := u.Query()
		for k, v := range urlParams {
			q.Set(k, v)
		}
		u.RawQuery = q.Encode()
	}
	uStr := u.String()
	kv := llog.KV{
		"url": uStr,
	}
	llog.Debug("making seaweed PUT request", kv)

	// we HAVE to upload a form the file in file
	newBody := &bytes.Buffer{}
	mpw := multipart.NewWriter(newBody)
	//todo: use content-type
	part, err := mpw.CreateFormFile("file", r.Filename())
	if err != nil {
		kv["error"] = err
		kv["filename"] = r.Filename()
		llog.Error("error creating multipart file", kv)
		return err
	}
	_, err = io.Copy(part, body)
	if err != nil {
		kv["error"] = err
		llog.Error("error copying body to multipart", kv)
		return err
	}
	err = mpw.Close()
	if err != nil {
		kv["error"] = err
		llog.Error("error closing multipart writer", kv)
		return err
	}

	req, err := http.NewRequest("PUT", uStr, newBody)
	if err != nil {
		kv["error"] = err
		llog.Warn("error making seaweed http request", kv)
		return err
	}
	req.Header.Add("Content-Type", mpw.FormDataContentType())
	var resp *http.Response
	var code int
	if resp, code, err = doReq(req, kv, http.StatusCreated); err != nil {
		if code == http.StatusNotFound {
			err = dhttp.NewError(code, "filename not found: %s", r.Filename())
		}
		return err
	}
	defer resp.Body.Close()
	return nil
}

// Lookup takes a filename and returns the seaweed url needed to get that file
func Lookup(filename string, urlParams map[string]string) (string, error) {
	fid, err := decodeFilename(filename)
	if err != nil {
		llog.Error("error decoding filename in lookup", llog.KV{
			"filename": filename,
			"error": err,
		})
		err = dhttp.NewError(http.StatusBadRequest,
			"invalid filename sent: %s", filename)
		return "", err
	}
	//fid's format is volumeId,somestuff
	parts := strings.Split(fid, ",")
	addr := srvclient.MaybeSRV(config.SeaweedAddr)
	uStr := "http://" + addr + "/dir/lookup?volumeId=" + parts[0]

	kv := llog.KV{
		"url":  uStr,
		"addr": addr,
	}
	llog.Debug("making seaweed GET request", kv)

	resp, err := http.Get(uStr)
	if err != nil {
		kv["error"] = err
		llog.Warn("error making seaweed http request", kv)
		return "", err
	}
	if code, err := handleResp(resp, kv, http.StatusOK); err != nil {
		if code == http.StatusNotFound {
			err = dhttp.NewError(code, "filename not found: %s", filename)
		}
		return "", err
	}
	defer resp.Body.Close()

	r := &lookupResult{}
	err = json.NewDecoder(resp.Body).Decode(r)
	if err != nil {
		kv["error"] = err
		llog.Error("error decoding get response from seaweed", kv)
		return "", err
	}
	if len(r.Locations) == 0 {
		err = dhttp.NewError(http.StatusNotFound,
			"filname not found: %s", filename)
		return "", err
	}
	i := rand.Intn(len(r.Locations))
	u := r.Locations[i].URL
	uStr = "http://" + u + "/" + fid + filepath.Ext(filename)

	if len(urlParams) > 0 {
		u, err := url.Parse(uStr)
		if err != nil {
			llog.Error("error building seaweed url", llog.KV{
				"url": uStr,
			})
			return "", err
		}
		vals := u.Query()
		for k, v := range urlParams {
			vals.Set(k, v)
		}
		u.RawQuery = vals.Encode()
		uStr = u.String()
	}

	return uStr, nil
}

// Get takes the given filename, gets the file from seaweed, returns an
// io.Reader you must close this io.Reader. The io.Reader might be nil if no
// response was returned or there was an error.
// You can also include headers HTTP headers to send along with the request
// and url params
func Get(filename string, headers, urlParams map[string]string) (io.ReadCloser, *http.Header, error) {
	uStr, err := Lookup(filename, urlParams)
	if err != nil {
		return nil, nil, err
	}

	kv := llog.KV{
		"url":      uStr,
		"filename": filename,
	}
	llog.Debug("making seaweed GET request", kv)

	resp, err := http.Get(uStr)
	if err != nil {
		kv["error"] = err
		llog.Warn("error making seaweed http request", kv)
		return nil, nil, err
	}
	for n, v := range headers {
		resp.Header.Set(n, v)
	}
	if code, err := handleResp(resp, kv, http.StatusOK, http.StatusNotModified, http.StatusRequestedRangeNotSatisfiable); err != nil {
		if code == http.StatusNotFound {
			err = dhttp.NewError(code, "filename not found: %s", filename)
		}
		return nil, &resp.Header, err
	}

	var r io.ReadCloser
	if resp.StatusCode == http.StatusOK {
		r = resp.Body
	}
	return r, &resp.Header, err
}

// Delete takes the given filename and deletes it from seaweed
func Delete(filename string) error {
	uStr, err := Lookup(filename, nil)
	if err != nil {
		return err
	}
	kv := llog.KV{
		"url":      uStr,
		"filename": filename,
	}
	llog.Debug("making seaweed DELETE request", kv)

	req, err := http.NewRequest("DELETE", uStr, nil)
	if err != nil {
		kv["error"] = err
		llog.Warn("error making seaweed http request", kv)
		return err
	}
	var resp *http.Response
	var code int
	if resp, code, err = doReq(req, kv, http.StatusAccepted); err != nil {
		if code == http.StatusNotFound {
			err = dhttp.NewError(code, "filename not found: %s", filename)
		}
		return err
	}
	defer resp.Body.Close()
	return nil
}
