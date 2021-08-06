package config

import (
	"encoding/json"
	"errors"
	"io"
	"io/ioutil"
	"os"

	"github.com/sirupsen/logrus"
)

type Config struct {
	Api struct {
		Addr string `json:"addr"`
	} `json:"api"`
	Proxy struct {
		Addr string `json:"addr"`
	} `json:"proxy"`
	Storage struct {
		Backing    string `json:"backing"`
		Filesystem struct {
			Directory string `json:"directory"`
		} `json:"filesystem"`
	} `json:"storage"`
	Log *logrus.Logger `json:"-"`
}

type ConfigOpts struct {
	AddrAPI   string
	AddrProxy string
	ConfFile  string
	Log       *logrus.Logger
}

func New(opts ConfigOpts) (Config, error) {
	c := Config{
		Log: &logrus.Logger{Out: ioutil.Discard},
	}
	// configure defaults
	c.Storage.Backing = "memory"
	c.Api.Addr = "127.0.0.1:8081"
	c.Proxy.Addr = "127.0.0.1:8080"

	// enable logging
	if opts.Log != nil {
		c.Log = opts.Log
	}

	// process config file
	if opts.ConfFile != "" {
		err := LoadFile(opts.ConfFile, &c)
		if err != nil {
			return c, err
		}
	}

	// TODO: read env

	// process cli flags
	if opts.AddrAPI != "" {
		c.Api.Addr = opts.AddrAPI
	}
	if opts.AddrProxy != "" {
		c.Proxy.Addr = opts.AddrProxy
	}

	return c, nil
}

func LoadReader(r io.Reader, c *Config) error {
	err := json.NewDecoder(r).Decode(c)
	if err != nil && !errors.Is(err, io.EOF) {
		return err
	}
	return nil
}

func LoadFile(filename string, c *Config) error {
	_, err := os.Stat(filename)
	if err != nil {
		return err
	}
	file, err := os.Open(filename)
	if err != nil {
		return err
	}
	defer file.Close()
	return LoadReader(file, c)
}
