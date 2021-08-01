package api

import (
	"fmt"
	"net/http"

	"github.com/sudo-bmitch/reproducible-proxy/config"
	"github.com/sudo-bmitch/reproducible-proxy/storage/backing"
)

// Start runs an api service
func Start(c config.Config, b backing.Backing) *http.Server {
	a := api{
		c: c,
		b: b,
	}
	handler := http.ServeMux{}
	handler.HandleFunc("/token/", a.createToken)
	server := http.Server{
		Handler: &handler,
		Addr:    c.Api.Addr,
	}

	go func() {
		err := server.ListenAndServe()
		if err != nil {
			a.c.Log.Fatal("ListenAndServe:", err)
		}
	}()

	return &server
}

type api struct {
	c config.Config
	b backing.Backing
}

// createToken
func (a *api) createToken(w http.ResponseWriter, req *http.Request) {
	// TODO: generate unique token
	fmt.Fprintf(w, "0")
}

// destroyToken

// createSnapshot

// metrics

// report

// status
