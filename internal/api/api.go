// Package api implements the handler for various API requests to httplock
package api

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"

	"github.com/gorilla/mux"
	"github.com/httplock/httplock/internal/api/docs"
	"github.com/httplock/httplock/internal/cert"
	"github.com/httplock/httplock/internal/config"
	"github.com/httplock/httplock/internal/storage"
	"github.com/httplock/httplock/ui"
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

// TODO: move storage types to separate package
type storageMetaResp struct {
	StatusCode int
	ContentLen int64
	Headers    http.Header
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
	uiFS, err := ui.GetFS()
	if err != nil {
		return nil, fmt.Errorf("failed initializing UI FS: %w", err)
	}

	r := mux.NewRouter()
	r.Handle("/", http.RedirectHandler("/ui/", http.StatusFound))
	r.HandleFunc("/api/ca", a.caGetPEM).Methods(http.MethodGet)
	r.HandleFunc("/api/token", a.tokenCreate).Methods(http.MethodPost)
	r.HandleFunc("/api/token/{id}", a.tokenDestroy).Methods(http.MethodDelete)
	r.HandleFunc("/api/token/{id}/save", a.tokenSave).Methods(http.MethodPost)
	r.HandleFunc("/api/root", a.rootList).Methods(http.MethodGet)
	r.HandleFunc("/api/root/{root}/dir", a.rootDir).Methods(http.MethodGet)
	r.HandleFunc("/api/root/{root}/file", a.rootFile).Methods(http.MethodGet)
	r.HandleFunc("/api/root/{root}/resp", a.rootResp).Methods(http.MethodGet)
	r.HandleFunc("/api/root/{root}/export", a.rootExport).Methods(http.MethodGet)
	r.HandleFunc("/api/root/{root}/import", a.rootImport).Methods(http.MethodPut)
	// r.HandleFunc("/api/root/{root}/path", a.rootPathHosts).Methods(http.MethodGet)
	// r.HandleFunc("/api/root/{root}/path/{host}", a.rootPathURLs).Methods(http.MethodGet)
	// r.HandleFunc("/api/root/{root}/path/{host}/{url}", a.rootPathReqs).Methods(http.MethodGet)
	// r.HandleFunc("/api/root/{root}/path/{host}/{url}/{req}/{type:(?:req-head|req-body|resp-head|resp-body)}", a.rootPathReq).Methods(http.MethodGet)
	// r.HandleFunc("/api/blob/{blob}", a.blobGet).Methods(http.MethodGet)
	r.PathPrefix("/swagger/").Handler(httpSwagger.Handler()).Methods(http.MethodGet)
	r.PathPrefix("/ui/").Handler(http.StripPrefix("/ui/", http.FileServer(http.FS(uiFS))))

	a.conf.Log.Println("Starting api server on", conf.API.Addr)
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
	caPEM, err := a.certs.CAGetPEM()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		a.conf.Log.Warnf("failed to get CA: %v", err)
		return
	}
	w.Header().Add("Content-Type", "application/text")
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
// @Param       hash query string false "hash used to initialize the response cache"
// @Success     201
// @Failure     500
// @Router      /api/token [post]
func (a *api) tokenCreate(w http.ResponseWriter, req *http.Request) {
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
		a.conf.Log.Warnf("failed to create token: %v", err)
		return
	}
	w.Header().Add("Content-Type", "application/json")
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
	id, ok := vars["id"]
	if !ok {
		w.WriteHeader(http.StatusBadRequest)
	}
	_, err := a.s.RootOpen(id)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		a.conf.Log.Warnf("failed to open root: %v", err)
	}

	// TODO: implement token delete
	w.Header().Add("Content-Type", "application/json")
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
		a.conf.Log.Warnf("failed to save root: %v", err)
		return
	}
	w.Header().Add("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	fmt.Fprintf(w, "{\"hash\": \"%s\"}", h)
}

// rootList returns a list of roots
// @Summary     Root List
// @Description Lists the roots
// @Produce     application/json
// @Success     200
// @Failure     500
// @Router      /api/root/ [get]
func (a *api) rootList(w http.ResponseWriter, req *http.Request) {
	index := a.s.Index()
	roots := []string{}
	for root := range index.Roots {
		roots = append(roots, root)
	}
	sort.Strings(roots)
	rootsJSON, err := json.Marshal(roots)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		a.conf.Log.Warnf("failed to marshal roots: %v", err)
	}
	w.Header().Add("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, err = w.Write(rootsJSON)
	if err != nil {
		a.conf.Log.Warnf("failed to write roots: %v", err)
	}
}

// rootDir returns the directory contents in a root fs
// @Summary     Root Dir
// @Description Lists a directory in a root
// @Produce     application/json
// @Param       root path  string   true  "root hash or uuid"
// @Param       path query []string false "path to list"
// @Success     200
// @Failure     400
// @Failure     500
// @Router      /api/root/{root}/dir [get]
func (a *api) rootDir(w http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	rootID, ok := vars["root"]
	if !ok {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	err := req.ParseForm()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		a.conf.Log.Warnf("failed to parse the form: %v", err)
		return
	}
	path, ok := req.Form["path"]
	if !ok || path == nil {
		path = []string{}
	}
	root, err := a.s.RootOpen(rootID)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		a.conf.Log.Warnf("failed to open root: %v", err)
		return
	}
	entries, err := root.List(path)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		a.conf.Log.Warnf("failed to list directory for root: %v", err)
		return
	}
	entriesJSON, err := json.Marshal(entries)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		a.conf.Log.Warnf("failed to marshal entries: %v", err)
	}
	w.Header().Add("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, err = w.Write(entriesJSON)
	if err != nil {
		a.conf.Log.Warnf("failed to write entries: %v", err)
	}
}

// rootFile returns file contents in a root fs
// @Summary     Root File
// @Description Get file contents in a root
// @Produce     application/octet-stream
// @Param       root path  string   true  "root hash or uuid"
// @Param       path query []string false "path of file"
// @Param       ct query string false "content-type to set on the returned file"
// @Success     200
// @Failure     400
// @Failure     500
// @Router      /api/root/{root}/file [get]
func (a *api) rootFile(w http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	rootID, ok := vars["root"]
	if !ok {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	err := req.ParseForm()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		a.conf.Log.Warnf("failed to parse the form: %v", err)
		return
	}
	path, ok := req.Form["path"]
	if !ok {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	ct := req.Form.Get("ct")
	if ct == "" {
		// default to octet-stream
		ct = "application/octet-stream"
	}
	root, err := a.s.RootOpen(rootID)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		a.conf.Log.Warnf("failed to open root: %v", err)
		return
	}
	rdr, err := root.Read(path)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		a.conf.Log.Warnf("failed to list directory for root: %v", err)
		return
	}
	w.Header().Add("Content-Type", ct)
	w.WriteHeader(http.StatusOK)
	_, err = io.Copy(w, rdr)
	if err != nil {
		a.conf.Log.Warnf("failed to write file: %v", err)
	}
}

// rootResp returns the response from a specific request
// @Summary     Root Response
// @Description Return the response from a request, including headers
// @Param       root path  string   true  "root hash or uuid"
// @Param       path query []string true "path of request"
// @Param       hash query string true "request hash"
// @Success     200
// @Failure     400
// @Failure     500
// @Router      /api/root/{root}/resp [get]
func (a *api) rootResp(w http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	rootID, ok := vars["root"]
	if !ok {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	err := req.ParseForm()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		a.conf.Log.Warnf("failed to parse the form: %v", err)
		return
	}
	path, ok := req.Form["path"]
	if !ok {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	hash := req.Form.Get("hash")
	if hash == "" {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	root, err := a.s.RootOpen(rootID)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		a.conf.Log.Warnf("failed to open root: %v", err)
		return
	}
	// read response headers
	// TODO: move "-resp-head" and "-resp-body" to const in a common package
	pathRespHead := append(path, hash+"-resp-head")
	rdrHead, err := root.Read(pathRespHead)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		a.conf.Log.Warnf("failed to list directory for root: %v", err)
		return
	}
	defer rdrHead.Close()
	metaResp := storageMetaResp{}
	err = json.NewDecoder(rdrHead).Decode(&metaResp)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		a.conf.Log.Warnf("failed to process headers from storage: %v", err)
		return
	}

	// open/copy response body
	pathRespBody := append(path, hash+"-resp-body")
	rdrBody, err := root.Read(pathRespBody)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		a.conf.Log.Warnf("failed to list directory for root: %v", err)
		return
	}
	defer rdrBody.Close()

	// copy the headers, write status, copy body
	wh := w.Header()
	for k, vv := range metaResp.Headers {
		for _, v := range vv {
			wh.Add(k, v)
		}
	}
	w.WriteHeader(metaResp.StatusCode)
	_, err = io.Copy(w, rdrBody)
	if err != nil {
		a.conf.Log.Warnf("failed to write body of response: %v", err)
	}
}

// // rootPathHosts returns a list of hosts in the root fs
// // @Summary     Root Path Hosts
// // @Description Lists the hosts in a root
// // @Produce     application/json
// // @Param       root path string true "root hash or uuid"
// // @Success     200
// // @Failure     400
// // @Failure     500
// // @Router      /api/root/{root}/path [get]
// func (a *api) rootPathHosts(w http.ResponseWriter, req *http.Request) {
// 	vars := mux.Vars(req)
// 	rootID, ok := vars["root"]
// 	if !ok {
// 		w.WriteHeader(http.StatusBadRequest)
// 		return
// 	}
// 	root, err := a.s.RootOpen(rootID)
// 	if err != nil {
// 		w.WriteHeader(http.StatusInternalServerError)
// 		a.conf.Log.Warnf("failed to open root: %v", err)
// 		return
// 	}
// 	entries, err := root.List([]string{})
// 	if err != nil {
// 		w.WriteHeader(http.StatusInternalServerError)
// 		a.conf.Log.Warnf("failed to list hosts for root: %v", err)
// 		return
// 	}
// 	hosts := []string{}
// 	for k := range entries {
// 		hosts = append(hosts, k)
// 	}
// 	sort.Strings(hosts)
// 	hostsJSON, err := json.Marshal(hosts)
// 	if err != nil {
// 		w.WriteHeader(http.StatusInternalServerError)
// 		a.conf.Log.Warnf("failed to marshal hosts: %v", err)
// 	}
// 	w.WriteHeader(http.StatusOK)
// 	_, err = w.Write(hostsJSON)
// 	if err != nil {
// 		a.conf.Log.Warnf("failed to write hosts: %v", err)
// 	}
// }

// // rootPathURLs returns a list of urls in the root fs
// // @Summary     Root Path URLs
// // @Description Lists the URLs for a given host
// // @Produce     application/json
// // @Param       root path string true "root hash or uuid"
// // @Param       host path string true "host/port"
// // @Success     200
// // @Failure     400
// // @Failure     500
// // @Router      /api/root/{root}/path/{host} [get]
// func (a *api) rootPathURLs(w http.ResponseWriter, req *http.Request) {
// 	vars := mux.Vars(req)
// 	rootID, ok := vars["root"]
// 	if !ok {
// 		w.WriteHeader(http.StatusBadRequest)
// 		return
// 	}
// 	host, ok := vars["host"]
// 	if !ok {
// 		w.WriteHeader(http.StatusBadRequest)
// 		return
// 	}
// 	root, err := a.s.RootOpen(rootID)
// 	if err != nil {
// 		w.WriteHeader(http.StatusInternalServerError)
// 		a.conf.Log.Warnf("failed to open root: %v", err)
// 		return
// 	}
// 	entries, err := root.List([]string{host})
// 	if err != nil {
// 		w.WriteHeader(http.StatusInternalServerError)
// 		a.conf.Log.Warnf("failed to list urls for host: %v", err)
// 		return
// 	}
// 	urls := []string{}
// 	for k := range entries {
// 		urls = append(urls, k)
// 	}
// 	sort.Strings(urls)
// 	urlsJSON, err := json.Marshal(urls)
// 	if err != nil {
// 		w.WriteHeader(http.StatusInternalServerError)
// 		a.conf.Log.Warnf("failed to marshal URLs: %v", err)
// 	}
// 	w.WriteHeader(http.StatusOK)
// 	_, err = w.Write(urlsJSON)
// 	if err != nil {
// 		a.conf.Log.Warnf("failed to write URLs: %v", err)
// 	}
// }

// // rootPathReqs returns a list of requests to a url in the root fs
// // @Summary     Root Path Requests
// // @Description Lists the requests for a given url
// // @Produce     application/json
// // @Param       root path string true "root hash or uuid"
// // @Param       host path string true "host/port"
// // @Param       url path string true "url"
// // @Success     200
// // @Failure     400
// // @Failure     500
// // @Router      /api/root/{root}/path/{host}/{url} [get]
// func (a *api) rootPathReqs(w http.ResponseWriter, req *http.Request) {
// 	vars := mux.Vars(req)
// 	rootID, ok := vars["root"]
// 	if !ok {
// 		w.WriteHeader(http.StatusBadRequest)
// 		return
// 	}
// 	host, ok := vars["host"]
// 	if !ok {
// 		w.WriteHeader(http.StatusBadRequest)
// 		return
// 	}
// 	url, ok := vars["url"]
// 	if !ok {
// 		w.WriteHeader(http.StatusBadRequest)
// 		return
// 	}
// 	root, err := a.s.RootOpen(rootID)
// 	if err != nil {
// 		w.WriteHeader(http.StatusInternalServerError)
// 		a.conf.Log.Warnf("failed to open root: %v", err)
// 		return
// 	}
// 	entries, err := root.List([]string{host, url})
// 	if err != nil {
// 		w.WriteHeader(http.StatusInternalServerError)
// 		a.conf.Log.Warnf("failed to list requests for url: %v", err)
// 		return
// 	}
// 	reqs := []string{}
// 	for k := range entries {
// 		if strings.HasSuffix(k, "-req-head") {
// 			reqID := strings.TrimSuffix(k, "-req-head")
// 			reqs = append(reqs, reqID)
// 		}
// 	}
// 	sort.Strings(reqs)
// 	reqsJSON, err := json.Marshal(reqs)
// 	if err != nil {
// 		w.WriteHeader(http.StatusInternalServerError)
// 		a.conf.Log.Warnf("failed to marshal reqs: %v", err)
// 	}
// 	w.WriteHeader(http.StatusOK)
// 	_, err = w.Write(reqsJSON)
// 	if err != nil {
// 		a.conf.Log.Warnf("failed to write reqs: %v", err)
// 	}
// }

// // rootPathReq returns a request to a url in the root fs
// // @Summary     Root Path Request
// // @Description Get a specific request to a given url
// // @Produce     application/json
// // @Param       root path string true "root hash or uuid"
// // @Param       host path string true "host/port"
// // @Param       url path string true "url"
// // @Param       req path string true "request hash"
// // @Param       type path string true "type of data from the request" Enums(req-head, req-body, resp-head, resp-body)
// // @Success     200
// // @Failure     400
// // @Failure     500
// // @Router      /api/root/{root}/path/{host}/{url}/{req}/{type} [get]
// func (a *api) rootPathReq(w http.ResponseWriter, req *http.Request) {
// 	vars := mux.Vars(req)
// 	rootID, ok := vars["root"]
// 	if !ok {
// 		w.WriteHeader(http.StatusBadRequest)
// 		return
// 	}
// 	host, ok := vars["host"]
// 	if !ok {
// 		w.WriteHeader(http.StatusBadRequest)
// 		return
// 	}
// 	url, ok := vars["url"]
// 	if !ok {
// 		w.WriteHeader(http.StatusBadRequest)
// 		return
// 	}
// 	reqHash, ok := vars["reqHash"]
// 	if !ok {
// 		w.WriteHeader(http.StatusBadRequest)
// 		return
// 	}
// 	reqType, ok := vars["type"]
// 	if !ok {
// 		w.WriteHeader(http.StatusBadRequest)
// 		return
// 	}
// 	root, err := a.s.RootOpen(rootID)
// 	if err != nil {
// 		w.WriteHeader(http.StatusInternalServerError)
// 		a.conf.Log.Warnf("failed to open root: %v", err)
// 		return
// 	}
// 	rdr, err := root.Read([]string{host, url, reqHash + "-" + reqType})
// 	if err != nil {
// 		w.WriteHeader(http.StatusInternalServerError)
// 		a.conf.Log.Warnf("failed to open file: %v", err)
// 		return
// 	}
// 	defer rdr.Close()
// 	w.WriteHeader(http.StatusOK)
// 	_, err = io.Copy(w, rdr)
// 	if err != nil {
// 		a.conf.Log.Warnf("failed to write file: %v", err)
// 	}
// }

// rootExport returns a tar.gz of a given hash
// @Summary     Root export
// @Description Exports a hash, returning a tar+gz
// @Produce     application/x-gtar
// @Param       root path string true "hash"
// @Success     200
// @Failure     400
// @Failure     500
// @Router      /api/root/{root}/export [get]
func (a *api) rootExport(w http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	// load the token and get the root
	rootID, ok := vars["root"]
	if !ok || strings.HasPrefix(rootID, "uuid:") {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	_, err := a.s.RootOpen(rootID)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
	}
	// create a tar writer
	w.Header().Add("Content-Type", "application/x-gtar")
	err = storage.Export(a.s, rootID, w)
	if err != nil {
		// WriteHeader may fail depending on where it failed
		w.WriteHeader(http.StatusInternalServerError)
		a.conf.Log.Warnf("failed to export: %v", err)
	}
}

// rootImport reads a tar.gz export into a new root hash
// @Summary     Root import
// @Description Imports a root hash from a tar+gz
// @Accept      application/x-gtar
// @Param       root path string true "hash"
// @Success     201
// @Failure     400
// @Failure     500
// @Router      /api/root/{root}/import [put]
func (a *api) rootImport(w http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	// check requested root
	rootID, ok := vars["root"]
	if !ok || strings.HasPrefix(rootID, "uuid:") {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	err := storage.Import(a.s, rootID, req.Body)
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
