package main

import (
	"encoding/json"
	"github.com/levenlabs/dank/config"
	dhttp "github.com/levenlabs/dank/http"
	"github.com/levenlabs/dank/seaweed"
	"github.com/levenlabs/dank/upload"
	"github.com/levenlabs/go-llog"
	"github.com/levenlabs/go-srvclient"
	"github.com/levenlabs/golib/rpcutil"
	"github.com/mediocregopher/skyapi/client"
	"mime"
	"net/http"
	"strings"
	"mime/multipart"
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
	http.HandleFunc("/get/", dhttp.WrapHandler(getPathHandler, "GET"))
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

func getHandler(w http.ResponseWriter, r *http.Request, args *getArgs) (int, error) {
	kv := rpcutil.RequestKV(r)
	kv["filename"] = args.Filename
	llog.Debug("received request to get", kv)

	if args.Filename == "" {
		return 404, nil
	}

	//todo: copy headers from seaweed?
	h, err := seaweed.Get(args.Filename, w)
	if err != nil {
		if err == seaweed.ErrorNotFound {
			return 404, nil
		}
		kv["error"] = err
		llog.Warn("error getting file", kv)
	} else {
		ct := h.Get("Content-Type")
		if ct != "" {
			w.Header().Set("Content-Type", ct)
		}
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
	Signature string `json:"sig" mapstructure:"sig"  validate:"nonzero"`
	Filename  string `json:"filename"  mapstructure:"filename" validate:"nonzero"`
	FormKey   string `json:"formKey" mapstructure:"form_key"`
}

type uploadRes struct {
	Filename string `json:"sig"`
	ContentType string `json:"contentType"`
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
		var bh *multipart.FileHeader
		body, bh, err = r.FormFile(args.FormKey)
		if err != nil {
			kv["key"] = args.FormKey
			kv["error"] = err
			llog.Warn("error getting the FormFile", kv)
			return 0, dhttp.NewError(http.StatusBadRequest, "error reading form key: %s", err.Error())
		}
		bct := bh.Header.Get("Content-Type")
		ct, _, _ = mime.ParseMediaType(bct)
	}

	a := &upload.Assignment{
		Signature: args.Signature,
		Filename:  args.Filename,
	}
	err = upload.Upload(a, body, r.ContentLength)
	if err != nil {
		kv["error"] = err
		llog.Warn("error uploading file", kv)
		return 0, err
	}

	js, err := json.Marshal(&uploadRes{
		Filename: args.Filename,
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

func verifyHandler(w http.ResponseWriter, r *http.Request, args *upload.Assignment) (int, error) {
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
		err := upload.Verify(&upload.Assignment{
			Signature: args.Signature,
			Filename:  args.Filename,
		})
		if err != nil {
			return 0, err
		}
	}

	err := seaweed.Delete(args.Filename)
	if err != nil {
		if err == seaweed.ErrorNotFound {
			return 404, nil
		}
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
