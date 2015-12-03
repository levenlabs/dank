package main
import (
	"net/http"
	"github.com/levenlabs/go-llog"
	"github.com/levenlabs/dank/config"
	. "github.com/levenlabs/dank/http"
"github.com/levenlabs/go-srvclient"
"github.com/mediocregopher/skyapi/client"
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

	llog.Info("starting http listening", llog.KV{"addr": addr})
	err := http.ListenAndServe(addr, nil)
	llog.Fatal("http listening failed", llog.KV{"addr": addr, "err": err})
}
