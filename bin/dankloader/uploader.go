package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/levenlabs/dank/upload"
	"github.com/levenlabs/go-srvclient"
	"github.com/mediocregopher/lever"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
)

func main() {
	l := lever.New("dankloader", nil)
	l.Add(lever.Param{
		Name:        "--dank-addr",
		Description: "address:port of the dank instance to upload to",
		Default:     "127.0.0.1:8333",
	})
	l.Parse()

	dankURL, _ := l.ParamStr("--dank-addr")

	ac := len(os.Args)
	if ac < 2 {
		log.Fatal("Usage: dankloader [folder or file to upload]")
	}
	fName := os.Args[ac-1]
	f, err := os.Open(fName)
	if err != nil {
		log.Fatalf("Error reading %s: %s", fName, err)
	}
	defer f.Close()
	fs, err := f.Stat()
	if err != nil {
		log.Fatalf("Error reading %s: %s", fName, err)
	}

	var files []string
	if fs.IsDir() {
		files = []string{}
		filepath.Walk(fName, func(p string, i os.FileInfo, err error) error {
			if err != nil {
				log.Fatalf("Error reading %s: %s", p, err)
			}
			if i.IsDir() {
				return nil
			}
			files = append(files, p)
			return nil
		})
	} else {
		files = []string{fName}
	}

	var nn string
	for _, n := range files {
		nn, err = makeDank(n, dankURL)
		if err != nil {
			log.Fatalf("error uploading %s: %s", n, err)
		}
		fmt.Printf("%s => %s\n", n, nn)
	}
}

func makeDank(f, durl string) (string, error) {
	durl = srvclient.MaybeSRV(durl)

	resp, err := http.Get("http://" + durl + "/assign")
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	a := &upload.Assignment{}
	d := json.NewDecoder(resp.Body)
	err = d.Decode(a)
	if err != nil {
		return "", err
	}

	newBody := &bytes.Buffer{}
	mpw := multipart.NewWriter(newBody)
	part, err := mpw.CreateFormFile("file", f)
	if err != nil {
		return "", err
	}
	fr, err := os.Open(f)
	if err != nil {
		return "", err
	}
	_, err = io.Copy(part, fr)
	if err != nil {
		return "", err
	}
	err = mpw.Close()
	if err != nil {
		return "", err
	}

	u, err := url.Parse("http://" + durl + "/upload")
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
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	var rf string
	if resp.StatusCode == http.StatusOK {
		rf = a.Filename
	} else {
		err = fmt.Errorf("unexpected code from dank: %d", resp.StatusCode)
	}
	return rf, err
}
