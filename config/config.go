package config

import (
	"io/ioutil"

	"github.com/sirupsen/logrus"
)

type Config struct {
	Api struct {
		Addr string
	}
	Proxy struct {
		Addr string
	}
	Storage struct {
		Backing string
	}
	Log *logrus.Logger `json:"-"`
}

type ConfigOpts struct {
	AddrAPI   string
	AddrProxy string
	ConfFile  string
	Log       *logrus.Logger
}

func New(opts ConfigOpts) Config {
	c := Config{
		Log: &logrus.Logger{Out: ioutil.Discard},
	}
	// configure defaults
	c.Storage.Backing = "memory"
	c.Api.Addr = "127.0.0.1:8081"
	c.Proxy.Addr = "127.0.0.1:8080"

	// TODO: read env

	if opts.Log != nil {
		c.Log = opts.Log
	}

	// process options
	if opts.ConfFile != "" {
		// TODO: read config file

	}
	if opts.AddrAPI != "" {
		c.Api.Addr = opts.AddrAPI
	}
	if opts.AddrProxy != "" {
		c.Proxy.Addr = opts.AddrProxy
	}

	return c
}
