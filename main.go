package main

import (
	"bytes"
	"encoding/json"
	"github.com/levenlabs/dank/config"
	dhttp "github.com/levenlabs/dank/http"
	"github.com/levenlabs/dank/seaweed"
	"github.com/levenlabs/dank/upload"
	"github.com/levenlabs/dank/types"
	"github.com/levenlabs/go-llog"
	"github.com/levenlabs/go-srvclient"
	"github.com/levenlabs/golib/rpcutil"
	"github.com/mediocregopher/skyapi/client"
	"github.com/vincent-petithory/dataurl"
	"io"
	"io/ioutil"
	"mime"
	"mime/multipart"
	"net/http"
	"strconv"
	"strings"
	"time"
)

func main() {
	addr := config.ListenAddr

	if config.SkyAPIAddr != "" {
		skyapiAddr := srvclient.MaybeSRV(config.SkyAPIAddr)
		kv := llog.KV{"skyapiAddr": skyapiAddr}
		llog.Info("connecting to skyapi", kv)

		go func() {
			kv["err"] = client.ProvideOpts(client.Opts{
				SkyAPIAddr:        skyapiAddr,
				Service:           "dank",
				ThisAddr:          addr,
				ReconnectAttempts: 3,
			})
			llog.Fatal("skyapi giving up reconnecting", kv)
		}()
	}

	// /get/ is needed to handle the filenames in the path
	http.HandleFunc("/get/", dhttp.WrapHandler(getPathHandler, "GET", "HEAD"))
	http.HandleFunc("/get", dhttp.WrapHandler(getHandler, "GET"))
	http.HandleFunc("/assign", dhttp.WrapHandler(assignHandler, "GET"))
	http.HandleFunc("/upload", dhttp.WrapHandler(uploadHandler, "POST", "PUT"))
	http.HandleFunc("/verify", dhttp.WrapHandler(verifyHandler, "GET"))
	http.HandleFunc("/delete", dhttp.WrapHandler(deleteHandler, "POST"))
	http.HandleFunc("/delete/", dhttp.WrapHandler(deletePathHandler, "DELETE"))

	llog.Info("starting http listening", llog.KV{"addr": addr})
	err := http.ListenAndServe(addr, nil)
	llog.Fatal("http listening failed", llog.KV{"addr": addr, "err": err})
}

type getArgs struct {
	Filename string `json:"filename" mapstructure:"filename"`
}

var headersToSend = []string{
	"If-Modified-Since",
	"Accept",
	"Accept-Encoding",
	"Range",
}

var headersToCopy = []string{
	"Content-Type",
	"Last-Modified",
	"Content-Encoding",
	"Content-Length",
	"Accept-Ranges",
	"Expires",
	"Cache-Control",
}

func getHandler(w http.ResponseWriter, r *http.Request, args *getArgs) (int, error) {
	kv := rpcutil.RequestKV(r)
	kv["filename"] = args.Filename
	kv["method"] = r.Method
	llog.Debug("received request to get", kv)

	if args.Filename == "" {
		return 404, nil
	}

	up := r.URL.Query()
	// don't pass on the filename param
	if up.Get("filename") == args.Filename {
		up.Del("filename")
	}

	code := 200
	urlParams := dhttp.FirstQueryVals(up)
	var err error
	var body io.ReadCloser
	if r.Method == "HEAD" && r.Header.Get("X-Upstream-Redirect") != "" {
		var surl string
		surl, err = seaweed.Lookup(args.Filename, urlParams)
		if err == nil {
			w.Header().Set("Location", surl)
			kv["url"] = surl
			llog.Debug("returning location for upstream redirect", kv)
			code = 307
		}
	} else {
		hs := map[string]string{}
		for _, n := range headersToSend {
			v := r.Header.Get(n)
			if v != "" {
				hs[n] = v
			}
		}

		var h *http.Header
		body, h, err = seaweed.Get(args.Filename, hs, urlParams)
		if err == nil {
			for _, n := range headersToCopy {
				v := h.Get(n)
				if v != "" {
					w.Header().Set(n, v)
				}
			}
		}
	}

	if err != nil {
		kv["error"] = err
		llog.Warn("error getting file", kv)
		return 0, err
	}

	if body != nil {
		defer body.Close()

		if r.Method == "GET" {
			_, err = io.Copy(w, body)
			if err != nil {
				kv["error"] = err
				llog.Error("error copying body to writer", kv)
			}
		}
	}
	return code, nil
}

func getPathHandler(w http.ResponseWriter, r *http.Request, args *getArgs) (int, error) {
	if args.Filename == "" {
		p := strings.Split(r.URL.Path, "/")
		if len(p) < 3 || p[2] == "" {
			return 404, nil
		}
		args.Filename = p[2]
	}
	return getHandler(w, r, args)
}

func assignHandler(w http.ResponseWriter, r *http.Request, args *types.AssignRequest) (int, error) {
	kv := rpcutil.RequestKV(r)
	kv["fileType"] = args.FileType
	kv["maxSize"] = args.MaxSize
	llog.Debug("received request to assign", kv)

	a, err := upload.Assign(args)
	if err != nil {
		kv["error"] = err
		llog.Warn("error getting assign", kv)
		return 0, err
	}
	js, err := json.Marshal(a)
	if err != nil {
		kv["error"] = err
		llog.Warn("error running json.Marshal for assign result", kv)
		// do not return the error to the client
		return http.StatusInternalServerError, nil
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write(js)
	return 0, nil
}

// since mapstructure doesn't support embedded structs, copying these here from
// upload.Assignment
type uploadArgs struct {
	Signature    string `json:"sig" mapstructure:"sig"  validate:"nonzero"`
	Filename     string `json:"filename"  mapstructure:"filename" validate:"nonzero"`
	LastModified string `json:"lastModified" mapstructure:"last_modified"`
	FormKey      string `json:"formKey" mapstructure:"form_key"`
}

type uploadRes struct {
	Filename    string `json:"filename"`
	ContentType string `json:"contentType"`
}

func uploadHandler(w http.ResponseWriter, r *http.Request, args *uploadArgs) (int, error) {
	kv := rpcutil.RequestKV(r)
	kv["length"] = r.ContentLength
	kv["filename"] = args.Filename
	kv["method"] = r.Method

	ct := r.Header.Get("Content-Type")
	kv["contentType"] = ct
	// http/request.go's parsePostForm doesn't care about err so we shouldn't
	mt, _, _ := mime.ParseMediaType(ct)
	kv["mimeType"] = mt
	llog.Debug("received request to upload file", kv)

	body := r.Body
	var err error
	switch mt {
	case "application/x-www-form-urlencoded":
		fallthrough
	case "multipart/form-data":
		if args.FormKey == "" {
			args.FormKey = "file"
		}
		llog.Debug("handling form-data", kv)
		var bh *multipart.FileHeader
		body, bh, err = r.FormFile(args.FormKey)
		if err != nil {
			kv["key"] = args.FormKey
			kv["error"] = err
			llog.Warn("error getting the FormFile", kv)
			return 0, dhttp.NewError(http.StatusBadRequest, "error reading form key: %s", err.Error())
		}
		ct = bh.Header.Get("Content-Type")
	case "application/data-url":
		du, err := dataurl.Decode(body)
		if err != nil {
			kv["error"] = err
			llog.Warn("error reading data-uri", kv)
			return 0, dhttp.NewError(http.StatusBadRequest, "error reading data-uri: %s", err.Error())
		}
		ct = du.ContentType()
		body = ioutil.NopCloser(bytes.NewReader(du.Data))
	}

	a := &types.Assignment{
		Signature: args.Signature,
		Filename:  args.Filename,
	}

	if args.LastModified == "" {
		args.LastModified = strconv.FormatInt(time.Now().Unix(), 10)
	}
	extra := map[string]string{
		"ts": args.LastModified,
	}
	err = upload.Upload(a, body, r.ContentLength, ct, extra)
	if err != nil {
		kv["error"] = err
		llog.Warn("error uploading file", kv)
		return 0, err
	}

	js, err := json.Marshal(&uploadRes{
		Filename:    args.Filename,
		ContentType: ct,
	})
	if err != nil {
		kv["error"] = err
		llog.Warn("error running json.Marshal for upload result", kv)
		// do not return the error to the client
		return http.StatusInternalServerError, nil
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write(js)
	return 0, err
}

func verifyHandler(w http.ResponseWriter, r *http.Request, args *types.Assignment) (int, error) {
	kv := rpcutil.RequestKV(r)
	kv["filename"] = args.Filename
	llog.Debug("received request to verify", kv)

	err := upload.Verify(args)
	return 0, err
}

type deleteArgs struct {
	Signature string `json:"sig" mapstructure:"sig"`
	Filename  string `json:"filename"  mapstructure:"filename"`
}

func deleteHandler(w http.ResponseWriter, r *http.Request, args *deleteArgs) (int, error) {
	kv := rpcutil.RequestKV(r)
	kv["filename"] = args.Filename
	llog.Debug("received request to delete", kv)

	if args.Filename == "" {
		return 404, nil
	}

	if args.Signature != "" {
		err := upload.Verify(&types.Assignment{
			Signature: args.Signature,
			Filename:  args.Filename,
		})
		if err != nil {
			return 0, err
		}
	}

	err := seaweed.Delete(args.Filename)
	if err != nil {
		kv["error"] = err
		llog.Warn("error deleting file", kv)
	}
	return 0, err
}

func deletePathHandler(w http.ResponseWriter, r *http.Request, args *deleteArgs) (int, error) {
	if args.Filename == "" {
		p := strings.Split(r.URL.Path, "/")
		if len(p) < 3 || p[2] == "" {
			return 404, nil
		}
		args.Filename = p[2]
	}
	return deleteHandler(w, r, args)
}
