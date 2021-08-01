package proxy

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"

	"github.com/sudo-bmitch/reproducible-proxy/hasher"
	"github.com/sudo-bmitch/reproducible-proxy/storage"
)

// TODO: add headers that should not be used in cache calculations
var excludeHeaders = []string{
	"X-Forwarded-For",
}

type storageMetaReq struct {
	Proto   string
	Method  string
	User    string
	Query   string
	Headers http.Header
	CL      int64
	Body    []byte // should this be a hash instead of raw content?
}

type storageMetaResp struct {
	StatusCode int
	Headers    http.Header
}

func includeHeader(header string) bool {
	for _, e := range excludeHeaders {
		if e == header {
			return false
		}
	}
	return true
}

// convert a request to a path
func storageGenDirPath(req *http.Request) ([]string, error) {
	// returned path consists of:
	// protocol (http/https)
	// host
	// url path/file
	// this is only for the directory, filename is a request hash
	pathEls := []string{req.Proto, req.URL.Host}
	urlPathEls := strings.Split(req.URL.Path, "/")
	pathEls = append(pathEls, urlPathEls...)

	return pathEls, nil
}

func storageGenFilename(req *http.Request) (string, error) {
	// returned path consists of:
	// request hash: method (get/head/post/put), query args, filtered headers
	hashItems := storageMetaReq{
		Proto:   req.Proto,
		Method:  req.Method,
		User:    req.URL.User.String(),
		Query:   req.URL.Query().Encode(),
		Headers: http.Header{},
		CL:      req.ContentLength,
	}

	// TODO: Request body, or a hash of that body, is needed

	for k := range req.Header {
		if includeHeader(k) {
			hashItems.Headers[k] = req.Header[k]
		}
	}
	j, err := json.Marshal(hashItems)
	if err != nil {
		return "", err
	}
	h, err := hasher.FromBytes(j)
	if err != nil {
		return "", err
	}
	return h, nil
}

// read the CF if it exists
func storageGetResp(req *http.Request, root *storage.Dir) (*http.Response, error) {
	dirElems, err := storageGenDirPath(req)
	if err != nil {
		return nil, err
	}
	curDir := root
	for _, el := range dirElems {
		nextDir, err := curDir.GetDir(el)
		if err != nil {
			return nil, fmt.Errorf("Not found: %w", err)
		}
		curDir = nextDir
	}
	// TODO: replace req.Body request with a buffered reader or similar, my reader can also just return the hash
	// TODO: defer reseting the reader if it's not closed so any error reverts the reader to a useable state
	fileName, err := storageGenFilename(req)
	if err != nil {
		return nil, err
	}
	if _, ok := curDir.Entries[fileName]; !ok {
		return nil, fmt.Errorf("Not found")
	}
	cf, err := curDir.GetComplex(fileName)
	if err != nil {
		return nil, err
	}

	resp := http.Response{
		Header: http.Header{},
	}

	// copy metadata into response
	respR, err := cf.Read("meta-resp")
	defer respR.Close()
	if err != nil {
		return nil, err
	}
	mrj, err := ioutil.ReadAll(respR)
	if err != nil {
		return nil, err
	}
	metaResp := storageMetaResp{}
	err = json.Unmarshal(mrj, &metaResp)
	if err != nil {
		return nil, err
	}
	if metaResp.Headers != nil {
		for k, vv := range metaResp.Headers {
			for _, v := range vv {
				resp.Header.Add(k, v)
			}
		}
	}
	if metaResp.StatusCode > 0 {
		resp.StatusCode = metaResp.StatusCode
		resp.Status = http.StatusText(metaResp.StatusCode)
	}

	// return reader for body
	respBodyR, err := cf.Read("body")
	if err != nil {
		return nil, err
	}
	resp.Body = respBodyR

	return &resp, nil
}

// write a CF based on the response
// request and response body will be read, these should be replaced with tee readers to process the data elsewhere
func storagePutResp(req *http.Request, resp *http.Response, root *storage.Dir) error {
	dirElems, err := storageGenDirPath(req)
	if err != nil {
		return err
	}
	fileName, err := storageGenFilename(req)
	if err != nil {
		return err
	}
	curDir := root
	for _, el := range dirElems {
		nextDir, err := curDir.GetDir(el)
		if err != nil {
			// create dir if GetDir fails
			nextDir, err = curDir.CreateDir(el)
		}
		if err != nil {
			return err
		}
		curDir = nextDir
	}
	if _, ok := curDir.Entries[fileName]; ok {
		return fmt.Errorf("Entry already exists")
	}
	cf, err := curDir.CreateComplex(fileName)

	// add meta data on the request for auditing
	reqW, err := cf.Write("meta-req")
	if err != nil {
		return err
	}
	defer reqW.Close()
	metaReq := storageMetaReq{
		Proto:   req.Proto,
		Method:  req.Method,
		User:    req.URL.User.String(),
		Query:   req.URL.Query().Encode(),
		Headers: req.Header,
		CL:      req.ContentLength,
	}
	// TODO: include request body (hash?)
	mrj, err := json.Marshal(metaReq)
	if err != nil {
		return err
	}
	_, err = reqW.Write(mrj)
	if err != nil {
		return err
	}

	// add meta data on the response
	respW, err := cf.Write("meta-resp")
	if err != nil {
		return err
	}
	defer respW.Close()
	metaResp := storageMetaResp{
		Headers:    resp.Header,
		StatusCode: resp.StatusCode,
	}
	mrj, err = json.Marshal(metaResp)
	if err != nil {
		return err
	}
	_, err = respW.Write(mrj)
	if err != nil {
		return err
	}

	// replace resp.Body with a tee reader to cache body contents
	respBodyW, err := cf.Write("body")
	if err != nil {
		return err
	}
	teeR := newTeeRC(resp.Body, respBodyW)
	resp.Body = teeR

	return nil
}
