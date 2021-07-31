package config

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
}

type ConfigOpts struct {
	AddrAPI   string
	AddrProxy string
	ConfFile  string
}

func New(opts ConfigOpts) Config {
	c := Config{}
	// configure defaults
	c.Storage.Backing = "memory"
	c.Api.Addr = "127.0.0.1:8081"
	c.Proxy.Addr = "127.0.0.1:8080"

	// TODO: read env

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
