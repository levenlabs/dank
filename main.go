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
	"github.com/mediocregopher/skyapi/client"
	"net/http"
)

func main() {
	addr := config.ListenAddr

	if config.SkyAPIAddr != "" {
		skyapiAddr, err := srvclient.SRV(config.SkyAPIAddr)
		if err != nil {
			llog.Fatal("srv lookup of skyapi failed", llog.KV{"err": err})
		}

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

	http.HandleFunc("/get", dHttp.WrapHandler(getHandler, "GET"))
	http.HandleFunc("/assign", dHttp.WrapHandler(assignHandler, "GET"))
	http.HandleFunc("/upload", dHttp.WrapHandler(uploadHandler, "POST"))
	http.HandleFunc("/verify", dHttp.WrapHandler(verifyHandler, "GET"))

	llog.Info("starting http listening", llog.KV{"addr": addr})
	err := http.ListenAndServe(addr, nil)
	llog.Fatal("http listening failed", llog.KV{"addr": addr, "err": err})
}

type GetArgs struct {
	Filename string `json:"filename" mapstructure:"filename" validate:"nonzero"`
}

func getHandler(w http.ResponseWriter, r *http.Request, args *GetArgs) (int, error) {
	//todo: support /get/filname.jpg additionally to ease nginx proxying
	//todo: copy headers from seaweed?
	err := seaweed.Get(args.Filename, w)
	return 0, err
}

func assignHandler(w http.ResponseWriter, r *http.Request, args *upload.AssignRequest) (int, error) {
	a, err := upload.Assign(args)
	if err != nil {
		return http.StatusInternalServerError, err
	}
	js, err := json.Marshal(a)
	if err != nil {
		return http.StatusInternalServerError, err
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write(js)
	return 0, nil
}

func uploadHandler(w http.ResponseWriter, r *http.Request, args *upload.Assignment) (int, error) {
	//todo: handle form uploads
	err := upload.Upload(args, r.Body, r.ContentLength)
	return 0, err
}

func verifyHandler(w http.ResponseWriter, r *http.Request, args *upload.Assignment) (int, error) {
	err := upload.Verify(args)
	if err != nil {
		return http.StatusBadRequest, fmt.Errorf("invalid filename sent: %s", err.Error())
	}
	return 0, nil
}
