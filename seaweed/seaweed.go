package seaweed
import (
	"github.com/levenlabs/dank/config"
	"github.com/levenlabs/go-llog"
	"net/http"
	"encoding/json"
	"fmt"
	"io"
	"encoding/base64"
	"strings"
"math/rand"
)

type AssignResult struct {
	FID string `json:"fid"`
	URL string `json:"url"`
}

type LookupResult struct {
	Locations []Location `json:"locations"`
}

type Location struct {
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

func (r *AssignResult) Filename() string {
	return base64.URLEncoding.EncodeToString([]byte(r.FID))
}

func decodeFilename(f string) (string, error) {
	parts := strings.Split(f, ".")
	fid, err := base64.URLEncoding.DecodeString(parts[0])
	if err != nil {
		return "", err
	}
	return string(fid), nil
}

func NewResult(url, filename string) (*AssignResult, error) {
	fid, err := decodeFilename(filename)
	if err != nil {
		return nil, err
	}
	return &AssignResult{
		FID: fid,
		URL: url,
	}, nil
}

func Assign() (*AssignResult, error) {
	resp, err := http.Post(assignURL, "", nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("unexpected non-200 post status: %s", resp.Status)
	}
	r := &AssignResult{}
	err = json.NewDecoder(resp.Body).Decode(r)
	if err != nil {
		return nil, err
	}
	return r, nil
}

func Upload(r *AssignResult, body io.Reader) error {
	req, err := http.NewRequest("PUT", "http://" + r.URL + "/" + r.FID, body)
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
	r := &LookupResult{}
	err = json.NewDecoder(resp.Body).Decode(r)
	if err != nil {
		return err
	}
	if len(r.Locations) == 0 {
		return fmt.Errorf("not found")
	}
	i := rand.Perm(len(r.Locations))[0]
	url := r.Locations[i].URL

	resp, err = http.Get("http://" + url + "/" + fid)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	_, err = io.Copy(w, resp.Body)
	return err
}
