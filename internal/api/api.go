// Package api implements the handler for various API requests to httplock
package api

import (
	"encoding/base64"
	"fmt"
	"net/http"
	"strings"

	"github.com/gorilla/mux"
	"github.com/httplock/httplock/internal/cert"
	"github.com/httplock/httplock/internal/config"
	"github.com/httplock/httplock/internal/storage"
)

type api struct {
	conf  config.Config
	certs *cert.Cert
	s     storage.Storage
}

// Start runs an api service
func Start(conf config.Config, s storage.Storage, certs *cert.Cert) (*http.Server, error) {
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
	r.HandleFunc("/storage/{id}/export", a.storageExport).Methods("GET")
	r.HandleFunc("/storage/{id}/import", a.storageImport).Methods("PUT")
	server := http.Server{
		Handler: r,
		Addr:    conf.API.Addr,
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
		name, _, err = a.s.RootCreateFrom(hash)
	} else {
		name, _, err = a.s.RootCreate()
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
	_, err := a.s.RootOpen(id)
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
		return
	}
	// SaveRoot generates the hash and saves to a list of roots
	root, err := a.s.RootOpen(id)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
	}
	h, err := a.s.RootSave(root)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		// TODO: properly escape error message, and probably don't pass through err
		fmt.Fprintf(w, "{\"error\": \"%v\"", err)
		return
	}
	w.WriteHeader(http.StatusCreated)
	fmt.Fprintf(w, "{\"hash\": \"%s\"}", h)
}

// storageExport
func (a *api) storageExport(w http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	// load the token and get the root
	id, ok := vars["id"]
	if !ok || strings.HasPrefix(id, "uuid:") {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	_, err := a.s.RootOpen(id)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
	}
	// create a tar writer
	w.Header().Add("Content-Type", "application/x-gtar")
	err = storage.Export(a.s, id, w)
	if err != nil {
		a.conf.Log.Warnf("failed to export: %w", err)
	}
}

// storageImport
func (a *api) storageImport(w http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	// check requested root
	id, ok := vars["id"]
	if !ok || strings.HasPrefix(id, "uuid:") {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	err := storage.Import(a.s, id, req.Body)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		a.conf.Log.Warnf("failed to import: %v", err)
		return
	}

	w.WriteHeader(http.StatusCreated)
}

// metrics

// report

// status
