package api

import (
	"encoding/base64"
	"fmt"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/sudo-bmitch/reproducible-proxy/config"
	"github.com/sudo-bmitch/reproducible-proxy/storage"
)

type api struct {
	c config.Config
	s *storage.Storage
}

// Start runs an api service
func Start(c config.Config, s *storage.Storage) (*http.Server, error) {
	a := api{
		c: c,
		s: s,
	}
	r := mux.NewRouter()
	r.HandleFunc("/token", a.tokenCreate).Methods("POST")
	r.HandleFunc("/token/{id}", a.tokenDestroy).Methods("DELETE")
	r.HandleFunc("/token/{id}/save", a.tokenSave).Methods("POST")
	server := http.Server{
		Handler: r,
		Addr:    c.Api.Addr,
	}

	go func() {
		err := server.ListenAndServe()
		if err != nil {
			a.c.Log.Fatal("ListenAndServe:", err)
		}
	}()

	return &server, nil
}

func (a *api) tokenHandle(w http.ResponseWriter, req *http.Request) {
	switch req.Method {
	case "POST":
		a.tokenCreate(w, req)
	case "DELETE":
		a.tokenDestroy(w, req)
	default:
		// error, unhandled
		w.WriteHeader(http.StatusNotFound)
	}
}

// tokenCreate
func (a *api) tokenCreate(w http.ResponseWriter, req *http.Request) {
	w.Header().Add("Content-Type", "application/json")
	name, _, err := a.s.NewRoot()
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
