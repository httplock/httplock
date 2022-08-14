package proxy

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/httplock/httplock/hasher"
	"github.com/httplock/httplock/internal/storage"
)

const (
	extReqHead  = "-req-head"
	extReqBody  = "-req-body"
	extRespHead = "-resp-head"
	extRespBody = "-resp-body"
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
	// host
	// url scheme/host/path/file
	// this is only for the directory, filename is a request hash
	u := *req.URL
	u.RawQuery = ""
	return []string{req.URL.Host, u.String()}, nil
}

// storageGenReqHash computes a hash on the request
func storageGenReqHash(req *http.Request) (string, error) {
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
		if req.GetBody != nil {
			body, err := req.GetBody()
			if err != nil {
				return "", fmt.Errorf("getBody: %w", err)
			}
			req.Body = body
		}
		hrc, err := newHashRC(req.Body)
		if err != nil {
			return "", fmt.Errorf("newHashRC: %w", err)
		}
		hashItems.BodyHash = hrc.h
		req.Body = hrc
		req.GetBody = hrc.GetReader
	}

	for k := range req.Header {
		if includeHeader(k) {
			hashItems.Headers[k] = req.Header[k]
		}
	}
	j, err := json.Marshal(hashItems)
	if err != nil {
		return "", fmt.Errorf("json marshal: %w", err)
	}
	h, err := hasher.FromBytes(j)
	if err != nil {
		return "", fmt.Errorf("hash from bytes: %w", err)
	}
	return h, nil
}

// storageGetResp returns the response if it's cached
func storageGetResp(req *http.Request, s storage.Storage, root *storage.Root) (*http.Response, error) {
	// hash must always be generated on the GetResp to replace the req body with a hashing version
	reqHash, err := storageGenReqHash(req)
	if err != nil {
		return nil, err
	}
	dirElems, err := storageGenDirPath(req)
	if err != nil {
		return nil, err
	}
	respHeadBR, err := root.Read(append(dirElems, reqHash+extRespHead))
	if err != nil {
		return nil, err
	}
	defer respHeadBR.Close()
	respBodyBR, err := root.Read(append(dirElems, reqHash+extRespBody))
	if err != nil {
		return nil, err
	}

	metaResp := storageMetaResp{}
	err = json.NewDecoder(respHeadBR).Decode(&metaResp)
	if err != nil {
		respBodyBR.Close()
		return nil, err
	}
	resp := http.Response{
		Header: http.Header{},
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
	resp.Body = respBodyBR

	return &resp, nil
}

// write a CF based on the response
// request and response body will be read, these should be replaced with tee readers to process the data elsewhere
func storagePutResp(req *http.Request, resp *http.Response, s storage.Storage, root *storage.Root) error {
	dirElems, err := storageGenDirPath(req)
	if err != nil {
		return fmt.Errorf("generating path: %w", err)
	}
	reqHash, err := storageGenReqHash(req)
	if err != nil {
		return fmt.Errorf("generating req hash: %w", err)
	}

	metaReq := storageMetaReq{
		Proto:   req.Proto,
		Method:  req.Method,
		User:    req.URL.User.String(),
		Query:   req.URL.Query().Encode(),
		Headers: req.Header,
		CL:      req.ContentLength,
	}
	// include request body hash
	// TODO: store entire request body, not just the hash, to allow future replay/diff
	if hbr, ok := req.Body.(*hashReadCloser); ok && hbr != nil {
		metaReq.BodyHash = hbr.h
	} else {
		return fmt.Errorf("body reader is not a hashReadCloser")
	}
	metaResp := storageMetaResp{
		Headers:    resp.Header,
		StatusCode: resp.StatusCode,
	}

	reqHeadBW, err := root.Write(append(dirElems, reqHash+extReqHead))
	if err != nil {
		return fmt.Errorf("root write for req head: %w", err)
	}
	err = json.NewEncoder(reqHeadBW).Encode(metaReq)
	reqHeadBW.Close()
	if err != nil {
		return fmt.Errorf("json encode req: %w", err)
	}

	respHeadBW, err := root.Write(append(dirElems, reqHash+extRespHead))
	if err != nil {
		return fmt.Errorf("root write for resp head: %w", err)
	}
	err = json.NewEncoder(respHeadBW).Encode(metaResp)
	respHeadBW.Close()
	if err != nil {
		return fmt.Errorf("json encode resp: %w", err)
	}

	// replace resp.Body with a tee reader to cache body contents
	respBodyBW, err := root.Write(append(dirElems, reqHash+extRespBody))
	if err != nil {
		return fmt.Errorf("root write for resp body: %w", err)
	}
	resp.Body = newTeeRC(resp.Body, respBodyBW)

	return nil
}
