// Package config parses the config file
package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"strings"

	"github.com/sirupsen/logrus"
)

type cfAction int

const (
	ActionStrip cfAction = iota
	ActionIgnore
)

// MarshalText converts an action to a string
func (a cfAction) MarshalText() ([]byte, error) {
	var s string
	switch a {
	default:
		s = ""
	case ActionStrip:
		s = "strip"
	case ActionIgnore:
		s = "ignore"
	}
	return []byte(s), nil
}

// UnmarshalText converts TLSConf from a string
func (a *cfAction) UnmarshalText(b []byte) error {
	switch strings.ToLower(string(b)) {
	default:
		return fmt.Errorf("unknown action value \"%s\"", b)
	case "strip":
		*a = ActionStrip
	case "ignore":
		*a = ActionIgnore
	}
	return nil
}

type Config struct {
	API     API            `json:"api"`
	Proxy   Proxy          `json:"proxy"`
	Storage Storage        `json:"storage"`
	Log     *logrus.Logger `json:"-"`
}
type API struct {
	Addr string `json:"addr"`
}
type Proxy struct {
	Addr    string   `json:"addr"`
	Filters []Filter `json:"filters"`
}
type Filter struct {
	URLPrefix  *url.URL            `json:"urlPrefix"`
	Method     string              `json:"method"`
	ReqHeader  map[string]cfAction `json:"reqHeader"`
	RespHeader map[string]cfAction `json:"respHeader"`
	ReqQuery   map[string]cfAction `json:"reqQuery"`
	BodyForm   map[string]cfAction `json:"bodyForm"`
}
type Storage struct {
	Kind      string `json:"kind"`
	Directory string `json:"directory"`
}

type ConfigOpts struct {
	AddrAPI   string
	AddrProxy string
	ConfFile  string
	Log       *logrus.Logger
}

func New(opts ConfigOpts) (Config, error) {
	c := Config{
		Log: &logrus.Logger{Out: io.Discard},
	}
	// configure defaults
	c.Storage.Kind = "memory"
	c.API.Addr = "127.0.0.1:8081"
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
		c.API.Addr = opts.AddrAPI
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
