package proxy

import (
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"

	"github.com/sudo-bmitch/reproducible-proxy/internal/cert"
	"github.com/sudo-bmitch/reproducible-proxy/internal/config"
	"github.com/sudo-bmitch/reproducible-proxy/internal/storage"
)

type proxyHTTP struct {
	p *proxy
}

type proxyConnect struct {
	p    *proxy
	root *storage.Dir
}

type proxy struct {
	conf    config.Config
	certs   *cert.Cert
	storage *storage.Storage
	client  *http.Client
}

// Start creates a new proxy service
func Start(conf config.Config, s *storage.Storage, certs *cert.Cert) (*http.Server, error) {
	pe := proxy{
		conf:    conf,
		certs:   certs,
		storage: s,
		client: &http.Client{
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse
			},
			Timeout: 0, // TODO: determine more appropriate Timeout, configurable?
		},
	}
	ph := proxyHTTP{
		p: &pe,
	}
	server := http.Server{
		Handler: &ph,
		Addr:    conf.Proxy.Addr,
	}

	ph.p.conf.Log.Println("Starting proxy server on", conf.Proxy.Addr)
	go func() {
		err := server.ListenAndServe()
		// TODO: err is always non-nil, ignore normal shutdown
		if err != nil {
			ph.p.conf.Log.Warn("ListenAndServe:", err)
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

func (p *proxy) serveWithCache(w http.ResponseWriter, req *http.Request, root *storage.Dir) {
	//http: Request.RequestURI can't be set in client requests.
	//http://golang.org/src/pkg/net/http/client.go
	req.RequestURI = ""

	delHopHeaders(req.Header)

	if clientIP, _, err := net.SplitHostPort(req.RemoteAddr); err == nil {
		appendHostToXForwardHeader(req.Header, clientIP)
	}

	// check if content is in cache
	resp, err := storageGetResp(req, root, p.storage.Backing)
	if err == nil {
		p.conf.Log.Println("Cache hit")
	}
	if err != nil {
		p.conf.Log.Printf("Cache miss: %s, %v", req.URL.String(), err)
		if p.client == nil {
			p.client = &http.Client{}
		}

		resp, err = p.client.Do(req)
		if err != nil {
			http.Error(w, "Server Error", http.StatusInternalServerError)
			p.conf.Log.Printf("serveWithCache: client.Do failed: %v", err)
			// TODO: cache connection failed errors?
			return
		}
		// store result in cache
		err = storagePutResp(req, resp, root, p.storage.Backing)
		if err != nil {
			p.conf.Log.Printf("Error on storagePutResp: %v\n", err)
		}
	}
	defer resp.Body.Close()

	p.conf.Log.Println(req.RemoteAddr, " ", resp.Status)

	delHopHeaders(resp.Header)

	copyHeader(w.Header(), resp.Header)
	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body)
	resp.Body.Close()
}

func (p *proxy) getAuth(req *http.Request) (*storage.Dir, error) {
	// use proxy auth to get the correct token
	auth := req.Header.Get("Proxy-Authorization")
	if auth == "" {
		return nil, fmt.Errorf("No proxy header found")
	}
	user, pass, err := checkAuthBasic(auth)
	if err != nil {
		return nil, fmt.Errorf("Check basic auth failed on \"%s\": %v\n", auth, err)
	}
	if user != "token" {
		return nil, fmt.Errorf("Auth user is not token: %s\n", user)
	}
	root, err := p.storage.GetRoot(pass)
	if err != nil {
		return nil, fmt.Errorf("Get root failed on \"%s\": %v\n", pass, err)
	}
	return root, nil
}

// handle direct http proxy requests
func (ph *proxyHTTP) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	ph.p.conf.Log.Printf("ph.ServeHTTP: %s %s %s", req.RemoteAddr, req.Method, req.URL)

	if req.Method == "CONNECT" {
		ph.handleConnect(w, req)
		return
	}

	if req.URL.Scheme != "http" && req.URL.Scheme != "https" {
		msg := "unsupported request"
		http.Error(w, msg, http.StatusBadRequest)
		ph.p.conf.Log.Println(msg)
		return
	}

	// use proxy auth to get the correct token
	root, err := ph.p.getAuth(req)
	if err != nil {
		ph.p.conf.Log.Println(err)
		requireAuthBasic(w)
		return
	}

	ph.p.serveWithCache(w, req, root)
}

func (ph *proxyHTTP) handleConnect(w http.ResponseWriter, req *http.Request) {
	// use proxy auth to get the correct token
	root, err := ph.p.getAuth(req)
	if err != nil {
		ph.p.conf.Log.Println(err)
		requireAuthBasic(w)
		return
	}

	// identify remote host and generate a tls certificate for that host
	name, _, err := net.SplitHostPort(req.Host)
	if err != nil {
		ph.p.conf.Log.Info("Unable to determine host from ", req.Host, ": ", err)
		http.Error(w, "no upstream", http.StatusServiceUnavailable)
		return
	}
	tmpCert, err := ph.p.certs.LeafCert([]string{name})
	if err != nil {
		ph.p.conf.Log.Info("Unable to generate cert: ", err)
		http.Error(w, "no upstream", http.StatusServiceUnavailable)
		return
	}
	tlsConf := tls.Config{
		Certificates: []tls.Certificate{*tmpCert},
	}
	// if SNI is used, this will update the certificate if needed
	tlsConf.GetCertificate = func(hello *tls.ClientHelloInfo) (*tls.Certificate, error) {
		ph.p.conf.Log.Printf("GetCertificate for hello: %s", hello.ServerName)
		return ph.p.certs.LeafCert([]string{hello.ServerName})
	}

	// send the raw I/O to tls
	wh, ok := w.(http.Hijacker)
	if !ok {
		ph.p.conf.Log.Warn("Failed to configure writer as a hijacker")
		http.Error(w, "connect unavailabe", http.StatusServiceUnavailable)
		return
	}
	raw, _, err := wh.Hijack()
	if !ok {
		ph.p.conf.Log.Warn("Failed to hijack: ", err)
		http.Error(w, "connect unavailabe", http.StatusServiceUnavailable)
		return
	}
	defer raw.Close()
	if _, err = raw.Write([]byte("HTTP/1.1 200 OK\r\n\r\n")); err != nil {
		ph.p.conf.Log.Warn("Failed to send connect ok: ", err)
		return
	}
	tlsConn := tls.Server(raw, &tlsConf)
	defer tlsConn.Close()

	// build a proxyConnect that handles requests over connect and maps to existing auth (root)
	pc := proxyConnect{
		p:    ph.p,
		root: root,
	}
	server := http.Server{
		Handler: &pc,
	}

	// create a listener with the tls connection
	cw := &connWait{
		Conn: tlsConn,
		done: make(chan int),
	}
	ll := &listenList{
		cs: []net.Conn{cw},
	}

	// handle serve in a goroutine
	err = server.Serve(ll)
	// TODO: check for any error other than closed, only log those
	pc.p.conf.Log.Infof("handleConnect: server finished %v", err)
	// server will finish as soon as listener stops returning new conns
	// wait for tlsConn to finish before returning (which triggers all the defer x.Close())
	cw.Wait()

	// TODO: graceful shutdown when parent http.Server is in Shutdown

}

func (pc *proxyConnect) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	// update all connect requests with scheme and host
	req.URL.Scheme = "https"
	req.URL.Host = req.Host

	pc.p.conf.Log.Infof("Serving connect request %v\n", req)

	pc.p.serveWithCache(w, req, pc.root)
}

type connWait struct {
	net.Conn
	done chan int
}

func (c *connWait) Close() error {
	close(c.done)
	return c.Conn.Close()
}

func (c *connWait) Wait() {
	<-c.done
}

type listenList struct {
	cs []net.Conn
	mu sync.Mutex
}

func (l *listenList) Accept() (net.Conn, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.cs == nil || len(l.cs) == 0 {
		return nil, fmt.Errorf("closed")
	}
	c := l.cs[0]
	l.cs = l.cs[1:]
	return c, nil
}

func (l *listenList) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.cs == nil || len(l.cs) == 0 {
		return nil
	}
	var err error
	for _, c := range l.cs {
		cErr := c.Close()
		if cErr != nil {
			err = cErr
		}
	}
	if err != nil {
		return err
	}
	return nil
}

func (l *listenList) Addr() net.Addr {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.cs == nil || len(l.cs) == 0 {
		return nil
	}
	return l.cs[0].LocalAddr()
}
