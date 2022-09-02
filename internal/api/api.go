// Package api implements the handler for various API requests to httplock
package api

import (
	"encoding/base64"
	"fmt"
	"net/http"
	"strings"

	"github.com/gorilla/mux"
	"github.com/httplock/httplock/internal/api/docs"
	"github.com/httplock/httplock/internal/cert"
	"github.com/httplock/httplock/internal/config"
	"github.com/httplock/httplock/internal/storage"
	httpSwagger "github.com/swaggo/http-swagger"
)

// @title        httplock API
// @version      0.1
// @description  HTTP reproducible proxy server.
// @contact.url  https://github.com/httplock/httplock
// @license.name Apache 2.0
// @license.url  http://www.apache.org/licenses/LICENSE-2.0.html

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
	if conf.API.Addr != "" {
		if conf.API.Addr[0] == ':' {
			docs.SwaggerInfo.Host = "localhost" + conf.API.Addr
		} else {
			docs.SwaggerInfo.Host = conf.API.Addr
		}
	} else {
		docs.SwaggerInfo.Host = "localhost"
	}

	r := mux.NewRouter()
	r.HandleFunc("/api/ca", a.caGetPEM).Methods(http.MethodGet)
	r.HandleFunc("/api/token", a.tokenCreate).Methods(http.MethodPost)
	r.HandleFunc("/api/token/{id}", a.tokenDestroy).Methods(http.MethodDelete)
	r.HandleFunc("/api/token/{id}/save", a.tokenSave).Methods(http.MethodPost)
	r.HandleFunc("/api/root/{id}/export", a.rootExport).Methods(http.MethodGet)
	r.HandleFunc("/api/root/{id}/import", a.rootImport).Methods(http.MethodPut)
	r.PathPrefix("/swagger/").Handler(httpSwagger.Handler()).Methods(http.MethodGet)

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

// caGetPEM returns the current public CA
// @Summary     Get CA
// @Description returns the public CA in PEM format
// @Produce     application/text
// @Success     200
// @Failure     500
// @Router      /api/ca [get]
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

// tokenCreate creates a new token for recording a session
// @Summary     Token create
// @Description returns a new uuid for recording a session
// @Produce     application/json
// @Success     201
// @Failure     500
// @Router      /api/token [post]
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

// tokenDestroy deletes a token from the list of valid uuids
// @Summary     Token delete
// @Description returns a new uuid for recording a session
// @Produce     application/json
// @Param       id path string true "uuid"
// @Success     202
// @Failure     400
// @Failure     500
// @Router      /api/token/{id} [delete]
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
// @Summary     Token save
// @Description Saves a uuid token, returning an immutable hash
// @Produce     application/json
// @Param       id path string true "uuid"
// @Success     201
// @Failure     400
// @Failure     500
// @Router      /api/token/{id}/save [post]
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

// rootExport returns a tar.gz of a given hash
// @Summary     Root export
// @Description Exports a hash, returning a tar+gz
// @Produce     application/x-gtar
// @Param       id path string true "hash"
// @Success     200
// @Failure     400
// @Failure     500
// @Router      /api/root/{id}/export [get]
func (a *api) rootExport(w http.ResponseWriter, req *http.Request) {
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
		// WriteHeader may fail depending on where it failed
		w.WriteHeader(http.StatusInternalServerError)
		a.conf.Log.Warnf("failed to export: %w", err)
	}
}

// rootImport reads a tar.gz export into a new root hash
// @Summary     Root import
// @Description Imports a root hash from a tar+gz
// @Accept      application/x-gtar
// @Param       id path string true "hash"
// @Success     201
// @Failure     400
// @Failure     500
// @Router      /api/root/{id}/import [put]
func (a *api) rootImport(w http.ResponseWriter, req *http.Request) {
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
