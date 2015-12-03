// Package config provides for all configurable parameters the instance can have
package config

import (
	"github.com/levenlabs/go-llog"
	"github.com/mediocregopher/lever"
)

// All possible configurable variables
var (
	ListenAddr  string
	SeaweedAddr string
	Secret      string
	SkyAPIAddr  string
	LogLevel    string
)

func init() {
	l := lever.New("dank", nil)
	l.Add(lever.Param{
		Name:        "--listen-addr",
		Description: "address:port to listen for http requests on, or just :port",
		Default:     ":8333",
	})
	l.Add(lever.Param{
		Name:        "--seaweed-addr",
		Description: "Address of master seaweed instance",
		Default:     "127.0.0.1:9333",
	})
	l.Add(lever.Param{
		Name:        "--secret",
		Description: "Secret used to sign the signature when uploading. Must be 16 characters.",
		Default:     "uShouldChangThis",
	})
	l.Add(lever.Param{
		Name:        "--skyapi-addr",
		Description: "Hostname of skyapi, to be looked up via a SRV request. Unset means don't register with skyapi",
	})
	l.Add(lever.Param{
		Name:        "--log-level",
		Description: "Minimum log level to show, either debug, info, warn, error, or fatal",
		Default:     "info",
	})
	l.Parse()

	ListenAddr, _ = l.ParamStr("--listen-addr")
	SeaweedAddr, _ = l.ParamStr("--seaweed-addr")
	Secret, _ = l.ParamStr("--secret")
	SkyAPIAddr, _ = l.ParamStr("--skyapi-addr")
	LogLevel, _ = l.ParamStr("--log-level")

	llog.SetLevelFromString(LogLevel)
}
