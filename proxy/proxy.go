package proxy

import (
	"io"
	"log"
	"net"
	"net/http"
	"strings"

	"github.com/sudo-bmitch/reproducible-proxy/config"
	"github.com/sudo-bmitch/reproducible-proxy/storage"
	"github.com/sudo-bmitch/reproducible-proxy/storage/backing"
)

// Start creates a new proxy service
func Start(c config.Config, backing backing.Backing) *http.Server {
	handler := &proxy{
		backing: backing,
	}
	dr, err := storage.DirNew(handler.backing)
	if err != nil {
		log.Fatal("defaultRoot:", err) // TODO: non-fatal?
	}
	handler.defaultRoot = dr
	server := http.Server{
		Handler: handler,
		Addr:    c.Proxy.Addr,
	}

	log.Println("Starting proxy server on", c.Proxy.Addr)
	go func() {
		err := server.ListenAndServe()
		if err != nil {
			log.Fatal("ListenAndServe:", err) // TODO: non-fatal?
		}
	}()

	return &server
}

// Hop-by-hop headers. These are removed when sent to the backend.
// http://www.w3.org/Protocols/rfc2616/rfc2616-sec13.html
var hopHeaders = []string{
	"Connection",
	"Keep-Alive",
	"Proxy-Authenticate",
	"Proxy-Authorization",
	"Te", // canonicalized version of "TE"
	"Trailers",
	"Transfer-Encoding",
	"Upgrade",
}

func copyHeader(dst, src http.Header) {
	for k, vv := range src {
		for _, v := range vv {
			dst.Add(k, v)
		}
	}
}

func delHopHeaders(header http.Header) {
	for _, h := range hopHeaders {
		header.Del(h)
	}
}

func appendHostToXForwardHeader(header http.Header, host string) {
	// If we aren't the first proxy retain prior
	// X-Forwarded-For information as a comma+space
	// separated list and fold multiple headers into one.
	if prior, ok := header["X-Forwarded-For"]; ok {
		host = strings.Join(prior, ", ") + ", " + host
	}
	header.Set("X-Forwarded-For", host)
}

type proxy struct {
	backing     backing.Backing
	defaultRoot *storage.Dir
}

func (p *proxy) ServeHTTP(wr http.ResponseWriter, req *http.Request) {
	log.Println(req.RemoteAddr, " ", req.Method, " ", req.URL)

	if req.URL.Scheme != "http" && req.URL.Scheme != "https" {
		msg := "unsupported protocal scheme " + req.URL.Scheme
		http.Error(wr, msg, http.StatusBadRequest)
		log.Println(msg)
		return
	}

	//http: Request.RequestURI can't be set in client requests.
	//http://golang.org/src/pkg/net/http/client.go
	req.RequestURI = ""

	delHopHeaders(req.Header)

	if clientIP, _, err := net.SplitHostPort(req.RemoteAddr); err == nil {
		appendHostToXForwardHeader(req.Header, clientIP)
	}

	// TODO: get the current root pointer based on proxy login (uuid/hash)
	root := p.defaultRoot

	// check if content is in cache
	resp, err := storageGetResp(req, root)
	if err == nil {
		log.Println("Cache hit")
	}
	if err != nil {
		log.Printf("Cache miss: %v", err)
		client := &http.Client{}

		resp, err = client.Do(req)
		if err != nil {
			http.Error(wr, "Server Error", http.StatusInternalServerError)
			log.Fatal("ServeHTTP:", err) // TODO: non-fatal
		}
		// store result in cache
		err = storagePutResp(req, resp, root)
		if err != nil {
			log.Printf("Error on storagePutResp: %v\n", err)
		}
	}
	defer resp.Body.Close()

	log.Println(req.RemoteAddr, " ", resp.Status)

	delHopHeaders(resp.Header)

	copyHeader(wr.Header(), resp.Header)
	wr.WriteHeader(resp.StatusCode)
	io.Copy(wr, resp.Body)
	resp.Body.Close()
}
