package proxy

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"

	"github.com/sudo-bmitch/reproducible-proxy/hasher"
	"github.com/sudo-bmitch/reproducible-proxy/storage"
	"github.com/sudo-bmitch/reproducible-proxy/storage/backing"
)

// TODO: add headers that should not be used in cache calculations
var excludeHeaders = []string{
	"X-Forwarded-For",
}

type storageMetaReq struct {
	Proto    string
	Method   string
	User     string
	Query    string
	Headers  http.Header
	CL       int64
	BodyHash string
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
	pathEls := []string{req.URL.Scheme, req.URL.Host}
	urlPathEls := strings.SplitAfter(strings.TrimPrefix(req.URL.Path, "/"), "/")
	// trim trailing blank entry
	if len(urlPathEls) > 0 && urlPathEls[len(urlPathEls)-1] == "" {
		urlPathEls = urlPathEls[:len(urlPathEls)-1]
	}
	pathEls = append(pathEls, urlPathEls...)

	return pathEls, nil
}

func storageGenFilename(req *http.Request, b backing.Backing) (string, error) {
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

	// get hash of request body
	if hrc, ok := req.Body.(*hashReadCloser); ok && hrc != nil {
		hashItems.BodyHash = hrc.h
	} else {
		hrc, err := newHashRC(req.Body, b)
		if err != nil {
			return "", err
		}
		hashItems.BodyHash = hrc.h
		req.Body = hrc
		req.GetBody = nil
	}

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
func storageGetResp(req *http.Request, root *storage.Dir, b backing.Backing) (*http.Response, error) {
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
	fileName, err := storageGenFilename(req, b)
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
func storagePutResp(req *http.Request, resp *http.Response, root *storage.Dir, b backing.Backing) error {
	dirElems, err := storageGenDirPath(req)
	if err != nil {
		return err
	}
	fileName, err := storageGenFilename(req, b)
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
	// include request body hash
	if hbr, ok := req.Body.(*hashReadCloser); ok && hbr != nil {
		metaReq.BodyHash = hbr.h
	} else {
		return fmt.Errorf("Body reader is not a hashReadCloser")
	}
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
