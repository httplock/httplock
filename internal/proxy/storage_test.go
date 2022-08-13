package proxy

import (
	"bytes"
	"io"
	"net/http"
	"net/url"
	"testing"

	"github.com/httplock/httplock/internal/config"
	"github.com/httplock/httplock/internal/storage"
)

func TestStorage(t *testing.T) {
	reqURL, _ := url.Parse("http://example.com/test")
	req := http.Request{
		Method:     "POST",
		Proto:      "HTTP/1.0",
		ProtoMajor: 1,
		ProtoMinor: 0,
		URL:        reqURL,
		Header:     http.Header{},
	}
	req.Header.Add("Host", "example.com")
	req.Header.Add("User-Agent", "test/agent")
	req.Header.Add("Accept", "*/*")
	reqBodyText := []byte(`{"data": "Greetings"}`)
	reqBodyRdr := bytes.NewReader(reqBodyText)
	req.Body = io.NopCloser(reqBodyRdr)
	req.ContentLength = int64(len(reqBodyText))

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
	resp.Body = io.NopCloser(respBodyRdr)
	resp.ContentLength = int64(len(respBodyText))

	c := config.Config{}
	c.Storage.Kind = "memory"
	sMem, err := storage.Get(c)
	if err != nil {
		t.Errorf("failed setting up storage: %v", err)
		return
	}
	_, root, err := sMem.RootCreate()
	if err != nil {
		t.Errorf("failed setting up root: %v", err)
		return
	}

	t.Run("GetMissing", func(t *testing.T) {
		getResp, err := storageGetResp(&req, sMem, root)
		if err == nil {
			t.Errorf("Get a missing value unexpected succeeded: %v", getResp)
			return
		}
		t.Logf("Expected cache miss: %v", err)
	})

	t.Run("PutResponse", func(t *testing.T) {
		// sending a request will close the body
		err = req.Body.Close()
		if err != nil {
			t.Errorf("Failed to close req body: %v", err)
		}
		err = storagePutResp(&req, &resp, sMem, root)
		if err != nil {
			t.Errorf("Failed to put response in cache: %v", err)
		}
		// body must be read and closed to finish storage
		_, err = io.ReadAll(resp.Body)
		if err != nil {
			t.Errorf("Failed to read resp: %v", err)
		}
		err = resp.Body.Close()
		if err != nil {
			t.Errorf("Failed to close resp body: %v", err)
		}
	})

	t.Run("GetResponse", func(t *testing.T) {
		getResp, err := storageGetResp(&req, sMem, root)
		if err != nil {
			t.Errorf("Failed to retrieve response: %v", err)
			return
		}
		if getResp.StatusCode != resp.StatusCode {
			t.Errorf("Response mismatch on StatusCode: got %d, expect %d", getResp.StatusCode, resp.StatusCode)
		}
	})
}
