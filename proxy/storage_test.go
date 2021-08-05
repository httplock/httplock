package proxy

import (
	"bytes"
	"io/ioutil"
	"net/http"
	"net/url"
	"testing"

	"github.com/sudo-bmitch/reproducible-proxy/config"
	"github.com/sudo-bmitch/reproducible-proxy/storage"
	"github.com/sudo-bmitch/reproducible-proxy/storage/backing"
)

func TestStorage(t *testing.T) {
	reqURL, _ := url.Parse("http://example.com/test")
	req := http.Request{
		Method:     "GET",
		Proto:      "HTTP/1.0",
		ProtoMajor: 1,
		ProtoMinor: 0,
		URL:        reqURL,
		Header:     http.Header{},
	}
	req.Header.Add("Host", "example.com")
	req.Header.Add("User-Agent", "test/agent")
	req.Header.Add("Accept", "*/*")

	resp := http.Response{
		StatusCode: 200,
		Proto:      "HTTP/1.0",
		ProtoMajor: 1,
		ProtoMinor: 0,
		Header:     http.Header{},
	}
	resp.Status = http.StatusText(resp.StatusCode)
	resp.Header.Add("Content-Type", "application/json")
	respBodyText := []byte(`{"data": "Hello world"}`)
	respBodyRdr := bytes.NewReader(respBodyText)
	resp.Body = ioutil.NopCloser(respBodyRdr)
	resp.ContentLength = int64(len(respBodyText))

	c := config.Config{}
	c.Storage.Backing = "memory"
	backing := backing.Get(c)
	root, _ := storage.DirNew(backing)

	t.Run("GetMissing", func(t *testing.T) {
		getResp, err := storageGetResp(&req, root, backing)
		if err == nil {
			t.Errorf("Get a missing value unexpected succeeded: %v", getResp)
			return
		}
		t.Logf("Expected cache miss: %v", err)
	})

	t.Run("PutResponse", func(t *testing.T) {
		err := storagePutResp(&req, &resp, root, backing)
		if err != nil {
			t.Errorf("Failed to put response in cache: %v", err)
		}
		// body must be read and closed to finish storage
		ioutil.ReadAll(resp.Body)
		resp.Body.Close()
	})

	t.Run("GetResponse", func(t *testing.T) {
		getResp, err := storageGetResp(&req, root, backing)
		if err != nil {
			t.Errorf("Failed to retrieve response: %v", err)
			return
		}
		if getResp.StatusCode != resp.StatusCode {
			t.Errorf("Response mismatch on StatusCode: got %d, expect %d", getResp.StatusCode, resp.StatusCode)
		}
	})
}
