package proxy

import (
	"encoding/base64"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"

	"github.com/sudo-bmitch/reproducible-proxy/config"
	"github.com/sudo-bmitch/reproducible-proxy/storage"
)

type proxy struct {
	c           config.Config
	s           *storage.Storage
	defaultRoot *storage.Dir // TODO: get rid of this?
}

// Start creates a new proxy service
func Start(c config.Config, s *storage.Storage) (*http.Server, error) {
	p := proxy{
		c: c,
		s: s,
	}
	_, dr, err := s.NewRoot()
	if err != nil {
		return nil, fmt.Errorf("Failed to create default root: %w", err)
	}
	p.defaultRoot = dr
	server := http.Server{
		Handler: &p,
		Addr:    c.Proxy.Addr,
	}

	p.c.Log.Println("Starting proxy server on", c.Proxy.Addr)
	go func() {
		err := server.ListenAndServe()
		if err != nil {
			p.c.Log.Fatal("ListenAndServe:", err) // TODO: non-fatal?
		}
	}()

	return &server, nil
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

func checkAuthBasic(auth string) (string, string, error) {
	authParts := strings.Split(auth, " ")
	if len(authParts) != 2 || authParts[0] != "Basic" {
		return "", "", fmt.Errorf("Basic auth header not found")
	}
	userpassRaw, err := base64.StdEncoding.DecodeString(authParts[1])
	if err != nil {
		return "", "", err
	}
	userpass := strings.SplitN(string(userpassRaw), ":", 2)
	if len(userpass) != 2 {
		return "", "", fmt.Errorf("Basic user/pass missing")
	}
	return userpass[0], userpass[1], nil
}

func requireAuthBasic(w http.ResponseWriter) {
	w.Header().Add("Proxy-Authenticate", "Basic")
	w.Header().Add("Proxy-Connection", "close")
	w.WriteHeader(http.StatusProxyAuthRequired)
	return
}

func (p *proxy) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	p.c.Log.Println(req.RemoteAddr, " ", req.Method, " ", req.URL)

	if req.URL.Scheme != "http" && req.URL.Scheme != "https" {
		msg := "unsupported protocal scheme " + req.URL.Scheme
		http.Error(w, msg, http.StatusBadRequest)
		p.c.Log.Println(msg)
		return
	}

	// use proxy auth to get the correct token
	auth := req.Header.Get("Proxy-Authorization")
	if auth == "" {
		p.c.Log.Println("No proxy header found")
		requireAuthBasic(w)
		return
	}
	user, pass, err := checkAuthBasic(auth)
	if err != nil {
		p.c.Log.Printf("Check basic auth failed on \"%s\": %v\n", auth, err)
		requireAuthBasic(w)
		return
	}
	if user != "token" {
		p.c.Log.Printf("Auth user is not token: %s\n", user)
		requireAuthBasic(w)
		return
	}
	root, err := p.s.GetRoot(pass)
	if err != nil {
		p.c.Log.Printf("Get root failed on \"%s\": %v\n", pass, err)
		requireAuthBasic(w)
		return
	}

	//http: Request.RequestURI can't be set in client requests.
	//http://golang.org/src/pkg/net/http/client.go
	req.RequestURI = ""

	delHopHeaders(req.Header)

	if clientIP, _, err := net.SplitHostPort(req.RemoteAddr); err == nil {
		appendHostToXForwardHeader(req.Header, clientIP)
	}

	// check if content is in cache
	resp, err := storageGetResp(req, root, p.s.Backing)
	if err == nil {
		p.c.Log.Println("Cache hit")
	}
	if err != nil {
		p.c.Log.Printf("Cache miss: %v", err)
		client := &http.Client{}

		resp, err = client.Do(req)
		if err != nil {
			http.Error(w, "Server Error", http.StatusInternalServerError)
			p.c.Log.Fatal("ServeHTTP:", err) // TODO: non-fatal
		}
		// store result in cache
		err = storagePutResp(req, resp, root, p.s.Backing)
		if err != nil {
			p.c.Log.Printf("Error on storagePutResp: %v\n", err)
		}
	}
	defer resp.Body.Close()

	p.c.Log.Println(req.RemoteAddr, " ", resp.Status)

	delHopHeaders(resp.Header)

	copyHeader(w.Header(), resp.Header)
	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body)
	resp.Body.Close()
}
