package main

import (
	"encoding/json"
	"fmt"
	"github.com/levenlabs/dank/config"
	dHttp "github.com/levenlabs/dank/http"
	"github.com/levenlabs/dank/seaweed"
	"github.com/levenlabs/dank/upload"
	"github.com/levenlabs/go-llog"
	"github.com/levenlabs/go-srvclient"
	"github.com/levenlabs/golib/rpcutil"
	"github.com/mediocregopher/skyapi/client"
	"mime"
	"net/http"
	"strings"
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
	http.HandleFunc("/get/", dHttp.WrapHandler(getPathHandler, "GET"))
	http.HandleFunc("/get", dHttp.WrapHandler(getHandler, "GET"))
	http.HandleFunc("/assign", dHttp.WrapHandler(assignHandler, "GET"))
	http.HandleFunc("/upload", dHttp.WrapHandler(uploadHandler, "POST"))
	http.HandleFunc("/verify", dHttp.WrapHandler(verifyHandler, "GET"))

	llog.Info("starting http listening", llog.KV{"addr": addr})
	err := http.ListenAndServe(addr, nil)
	llog.Fatal("http listening failed", llog.KV{"addr": addr, "err": err})
}

type getArgs struct {
	Filename string `json:"filename" mapstructure:"filename"`
}

func getHandler(w http.ResponseWriter, r *http.Request, args *getArgs) (int, error) {
	kv := rpcutil.RequestKV(r)
	kv["filename"] = args.Filename
	llog.Debug("received request to get", kv)

	if args.Filename == "" {
		return 404, nil
	}

	//todo: copy headers from seaweed?
	err := seaweed.Get(args.Filename, w)
	if err != nil {
		kv["error"] = err
		llog.Warn("error getting file", kv)
	}
	return 0, err
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

func assignHandler(w http.ResponseWriter, r *http.Request, args *upload.AssignRequest) (int, error) {
	kv := rpcutil.RequestKV(r)
	kv["fileType"] = args.FileType
	kv["maxSize"] = args.MaxSize
	llog.Debug("received request to assign", kv)

	a, err := upload.Assign(args)
	if err != nil {
		kv["error"] = err
		llog.Warn("error getting assign", kv)
		return http.StatusInternalServerError, err
	}
	js, err := json.Marshal(a)
	if err != nil {
		kv["error"] = err
		llog.Warn("error running json.Marshal for assign result", kv)
		return http.StatusInternalServerError, err
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write(js)
	return 0, nil
}

// since mapstructure doesn't support embedded structs, copying these here from
// upload.Assignment
type uploadArgs struct {
	Signature string `json:"sig" mapstructure:"sig"  validate:"nonzero"`
	Filename  string `json:"filename"  mapstructure:"filename" validate:"nonzero"`
	FormKey   string `json:"formKey" mapstructure:"formKey"`
}

func uploadHandler(w http.ResponseWriter, r *http.Request, args *uploadArgs) (int, error) {
	kv := rpcutil.RequestKV(r)
	kv["length"] = r.ContentLength
	kv["filename"] = args.Filename

	ct := r.Header.Get("Content-Type")
	// http/request.go's parsePostForm doesn't care about err so we shouldn't
	ct, _, _ = mime.ParseMediaType(ct)
	kv["contentType"] = ct
	llog.Debug("received request to upload file", kv)

	body := r.Body
	var err error
	switch ct {
	case "application/x-www-form-urlencoded":
		fallthrough
	case "multipart/form-data":
		if args.FormKey == "" {
			args.FormKey = "file"
		}
		llog.Debug("handling form-data", kv)
		body, _, err = r.FormFile(args.FormKey)
		if err != nil {
			kv["key"] = args.FormKey
			kv["error"] = err
			llog.Warn("error getting the FormFile", kv)
			return http.StatusBadRequest, fmt.Errorf("error reading form key: %s", err.Error())
		}
	}

	a := &upload.Assignment{
		Signature: args.Signature,
		Filename:  args.Filename,
	}
	err = upload.Upload(a, body, r.ContentLength)
	if err != nil {
		kv["error"] = err
		llog.Warn("error uploading file", kv)
	}
	return 0, err
}

func verifyHandler(w http.ResponseWriter, r *http.Request, args *upload.Assignment) (int, error) {
	kv := rpcutil.RequestKV(r)
	kv["filename"] = args.Filename
	llog.Debug("received request to verify", kv)

	err := upload.Verify(args)
	if err != nil {
		return http.StatusBadRequest, fmt.Errorf("invalid filename sent: %s", err.Error())
	}
	return 0, nil
}
