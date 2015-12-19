// The dank package provides a Client for interfacing with an instance of dank.
package dank

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/levenlabs/dank/types"
	"github.com/levenlabs/go-srvclient"
	"io"
	"io/ioutil"
	"mime"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
)

type Client struct {
	hostname string
}

// AssignOptions mirrors an AssignRequest but can be constructed without
// requiring the dank/types package
type AssignOptions struct {
	types.AssignRequest
}

// NewClient retuns a new dank Client for the given hostname, the hostname will
// be looked up via an optimistic (if it fails we don't care) SRV record before
// each request.
func NewClient(hostname string) *Client {
	return &Client{
		hostname: hostname,
	}
}

func (d *Client) resolve() string {
	return srvclient.MaybeSRV(d.hostname)
}

// createFormFile is multipart.Writer.CreateFormFile but it detects the mime
func createFormFile(w *multipart.Writer, fieldname, filename string) (io.Writer, error) {
	h := make(textproto.MIMEHeader)
	h.Set("Content-Disposition",
		fmt.Sprintf(`form-data; name=%s; filename=%s`,
			strconv.Quote(fieldname), strconv.Quote(filename)))

	var ct string
	//try to determine the Content-Type from the filename
	ext := filepath.Ext(filename)
	if ext != "" {
		ct = mime.TypeByExtension(ext)
	}
	if ct == "" {
		ct = "application/octet-stream"
	}
	h.Set("Content-Type", ct)
	return w.CreatePart(h)
}

// Upload uploads an array of bytes and uploads it to the assignment. If
// assignment is nil, one will be created
//
// Returns the filename uploaded to and error.
func (d *Client) Upload(body []byte, a *types.Assignment) (string, error) {
	var err error
	if a == nil {
		a, err = d.Assign(nil)
		if err != nil {
			return "", err
		}
	}

	newBody := &bytes.Buffer{}
	mpw := multipart.NewWriter(newBody)
	part, err := createFormFile(mpw, "file", a.Filename)
	if err != nil {
		return "", err
	}
	if _, err = part.Write(body); err != nil {
		return "", err
	}
	if err = mpw.Close(); err != nil {
		return "", err
	}

	u, err := url.Parse("http://" + d.resolve() + "/upload")
	if err != nil {
		return "", err
	}
	q := u.Query()
	q.Set("sig", a.Signature)
	q.Set("filename", a.Filename)
	u.RawQuery = q.Encode()

	req, err := http.NewRequest("PUT", u.String(), newBody)
	if err != nil {
		return "", err
	}
	req.Header.Add("Content-Type", mpw.FormDataContentType())
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("unexpected code from dank: %d", resp.StatusCode)
	}
	return a.Filename, err
}

// UploadFile takes a diskFilename and reads the file off the disk and uploads
// it using Upload
func (d *Client) UploadFile(diskFilename string, a *types.Assignment) (string, error) {
	fr, err := os.Open(diskFilename)
	if err != nil {
		return "", err
	}
	b, err := ioutil.ReadAll(fr)
	if err != nil {
		return "", err
	}
	return d.Upload(b, a)
}

// Assign gets a assignment from seaweed
// If you want to have no restrictions/options on the file, send a nil
// AssignOptions
func (d *Client) Assign(ar *AssignOptions) (*types.Assignment, error) {
	u, err := url.Parse("http://" + d.resolve() + "/assign")
	if err != nil {
		return nil, err
	}
	if ar != nil {
		u.RawQuery = ar.URLValues().Encode()
	}

	resp, err := http.Get(u.String())
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	a := &types.Assignment{}
	dec := json.NewDecoder(resp.Body)
	err = dec.Decode(a)
	return a, err
}

// Verify takes an Assignment and validates that the signature matches a valid
// filename
func (d *Client) Verify(a *types.Assignment) error {
	u, err := url.Parse("http://" + d.resolve() + "/verify")
	if err != nil {
		return err
	}
	q := u.Query()
	q.Set("sig", a.Signature)
	q.Set("filename", a.Filename)
	u.RawQuery = q.Encode()

	resp, err := http.Get(u.String())
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected code from dank: %d", resp.StatusCode)
	}
	return nil
}
