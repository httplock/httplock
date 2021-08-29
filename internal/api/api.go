package api

import (
	"encoding/base64"
	"fmt"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/sudo-bmitch/reproducible-proxy/internal/cert"
	"github.com/sudo-bmitch/reproducible-proxy/internal/config"
	"github.com/sudo-bmitch/reproducible-proxy/internal/storage"
)

type api struct {
	conf  config.Config
	certs *cert.Cert
	s     *storage.Storage
}

// Start runs an api service
func Start(conf config.Config, s *storage.Storage, certs *cert.Cert) (*http.Server, error) {
	a := api{
		conf:  conf,
		certs: certs,
		s:     s,
	}
	r := mux.NewRouter()
	r.HandleFunc("/ca", a.caGetPEM).Methods("GET")
	r.HandleFunc("/token", a.tokenCreate).Methods("POST")
	r.HandleFunc("/token/{id}", a.tokenDestroy).Methods("DELETE")
	r.HandleFunc("/token/{id}/save", a.tokenSave).Methods("POST")
	server := http.Server{
		Handler: r,
		Addr:    conf.Api.Addr,
	}

	go func() {
		err := server.ListenAndServe()
		// TODO: err is always non-nil, ignore normal shutdown
		if err != nil {
			a.conf.Log.Warn("ListenAndServe:", err)
		}
	}()

	return &server, nil
}

func (a *api) caGetPEM(w http.ResponseWriter, req *http.Request) {
	w.Header().Add("Content-Type", "application/text")
	caPEM, err := a.certs.CAGetPEM()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
	_, err = w.Write(caPEM)
	if err != nil {
		a.conf.Log.Warn("Failed to send CA PEM: ", err)
	}
}

// tokenCreate
func (a *api) tokenCreate(w http.ResponseWriter, req *http.Request) {
	w.Header().Add("Content-Type", "application/json")
	// check for base hash arg, attempt to retrieve that instead of creating a NewRoot
	hash := req.FormValue("hash")
	var name string
	var err error
	if hash != "" {
		name, _, err = a.s.NewRootFrom(hash)
	} else {
		name, _, err = a.s.NewRoot()
	}
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		// TODO: properly escape error message, and probably don't pass through err
		fmt.Fprintf(w, "{\"error\": \"%v\"", err)
		return
	}
	w.WriteHeader(http.StatusCreated)
	token := base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("token:%s", name)))
	fmt.Fprintf(w, "{\"uuid\": \"%s\", \"auth\": \"%s\"}", name, token)
}

// tokenDestroy
func (a *api) tokenDestroy(w http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	w.Header().Add("Content-Type", "application/json")
	id, ok := vars["id"]
	if !ok {
		w.WriteHeader(http.StatusBadRequest)
	}
	_, err := a.s.GetRoot(id)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
	}

	// TODO: implement token delete
	w.WriteHeader(http.StatusNotImplemented)
}

// tokenSave: generates a hash and stores as a root
func (a *api) tokenSave(w http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	w.Header().Add("Content-Type", "application/json")
	id, ok := vars["id"]
	if !ok {
		w.WriteHeader(http.StatusBadRequest)
	}
	// SaveRoot generates the hash and saves to a list of roots
	h, err := a.s.SaveRoot(id)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		// TODO: properly escape error message, and probably don't pass through err
		fmt.Fprintf(w, "{\"error\": \"%v\"", err)
		return
	}
	w.WriteHeader(http.StatusCreated)
	fmt.Fprintf(w, "{\"hash\": \"%s\"}", h)
}

// metrics

// report

// status
