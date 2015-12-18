// The dank package provides a Client for interfacing with an instance of dank.
package dank

import (
	"github.com/levenlabs/go-srvclient"
	"github.com/levenlabs/dank/types"
	"net/http"
	"encoding/json"
	"mime/multipart"
	"bytes"
	"net/url"
	"fmt"
	"os"
	"io/ioutil"
)

type Client struct {
	hostname string
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

// Upload uploads an array of bytes and uploads it to filename. If filename is
// empty then a new filename will be created and uploaded. If you need verify
// the signature before uploading, you need to call Verify before calling
// Upload.
//
// Returns the filename uploaded to and error.
func (d *Client) Upload(body []byte, filename string) (string, error) {
	if filename == "" {
		a, err := d.Assign()
		if err != nil {
			return "", err
		}
		filename = a.Filename
	}

	newBody := &bytes.Buffer{}
	mpw := multipart.NewWriter(newBody)
	part, err := mpw.CreateFormFile("file", filename)
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
	q.Set("filename", filename)
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
	return filename, err
}

// UploadFile takes a diskFilename and reads the file off the disk and uploads
// it using Upload
func (d *Client) UploadFile(diskFilename, filename string) (string, error) {
	fr, err := os.Open(diskFilename)
	if err != nil {
		return "", err
	}
	b, err := ioutil.ReadAll(fr)
	if err != nil {
		return "", err
	}
	return d.Upload(b, filename)
}

// Assign gets a assignment from seaweed
func (d *Client) Assign() (*types.Assignment, error) {
	resp, err := http.Get("http://" + d.resolve() + "/assign")
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
