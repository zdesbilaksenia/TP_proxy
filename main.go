package main

import (
	"bufio"
	"crypto/tls"
	"github.com/gorilla/mux"
	"io"
	"log"
	"math/rand"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"os/exec"
	"strconv"
)

type RequestHandler struct {
	router *mux.Router
}

type HTTPSHandler struct {
	connRequest   *http.Request
	clientRequest *http.Request
	parsedURL     *url.URL
	config        *tls.Config
	clientConn    net.Conn
	serverConn    *tls.Conn
	response      *http.Response
}

func (h *RequestHandler) HandleRequest(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodConnect {
		parsedUrl, err := url.Parse(r.RequestURI)
		if err != nil {
			log.Println(err)
		}
		httpsHandler := &HTTPSHandler{
			connRequest: r,
			parsedURL:   parsedUrl,
		}
		h.HandleHTTPSRequest(w, r, httpsHandler)
		defer httpsHandler.Close()
		return
	}

	h.HandleHTTPRequest(w, r)
}

func (httpsHandler *HTTPSHandler) Close() {
	httpsHandler.clientConn.Close()
	httpsHandler.serverConn.Close()
	httpsHandler.response.Body.Close()
}

func (h *RequestHandler) HandleHTTPSRequest(w http.ResponseWriter, r *http.Request, httpsHandler *HTTPSHandler) {
	err := httpsHandler.GenClientCert()
	if err != nil {
		log.Println(err)
	}

	err = httpsHandler.MakeHttpsClientConn(w)
	if err != nil {
		log.Println(err)
	}

	err = httpsHandler.MakeHttpsServerConn()
	if err != nil {
		log.Println(err)
	}

	err = httpsHandler.GetHttpsRequest()
	if err != nil {
		log.Println(err)
	}

	err = httpsHandler.SendClientHTTPSRequest()
	if err != nil {
		log.Println(err)
	}

	err = httpsHandler.GetServerHTTPSResponse()
	if err != nil {
		log.Println(err)
	}
}

func (httpsHandler *HTTPSHandler) GenClientCert() error {
	pwd, err := os.Getwd()
	if err != nil {
		return err
	}

	genCertsDir := pwd + "/gen_certs"
	certsDir := genCertsDir + "/certs/"
	certFilename := certsDir + httpsHandler.parsedURL.Scheme + ".crt"

	_, errStat := os.Stat(certFilename)
	if os.IsNotExist(errStat) {
		genCommand := exec.Command(genCertsDir+"/gen_cert.sh", httpsHandler.parsedURL.Scheme, strconv.Itoa(rand.Intn(1000)), certsDir)
		_, err := genCommand.CombinedOutput()
		if err != nil {
			return err
		}
	}

	cert, err := tls.LoadX509KeyPair(certFilename, genCertsDir+"/cert.key")
	if err != nil {
		return err
	}

	config := new(tls.Config)
	config.Certificates = []tls.Certificate{cert}
	config.ServerName = httpsHandler.parsedURL.Scheme

	httpsHandler.config = config

	return nil
}

func (httpsHandler *HTTPSHandler) MakeHttpsClientConn(w http.ResponseWriter) error {
	raw, _, err := w.(http.Hijacker).Hijack()
	if err != nil {
		return err
	}

	_, err = raw.Write([]byte("HTTP/1.1 200 Connection established\r\n\r\n"))
	if err != nil {
		raw.Close()
		return err
	}

	clientConn := tls.Server(raw, httpsHandler.config)
	err = clientConn.Handshake()
	if err != nil {
		clientConn.Close()
		raw.Close()
		return err
	}

	httpsHandler.clientConn = clientConn

	return nil
}

func (httpsHandler *HTTPSHandler) MakeHttpsServerConn() error {
	serverConnection, err := tls.Dial("tcp", httpsHandler.connRequest.Host, httpsHandler.config)
	if err != nil {
		return err
	}

	httpsHandler.serverConn = serverConnection

	return nil
}

func (httpsHandler *HTTPSHandler) GetHttpsRequest() error {
	reader := bufio.NewReader(httpsHandler.clientConn)
	request, err := http.ReadRequest(reader)
	if err != nil {
		return err
	}

	httpsHandler.clientRequest = request

	return nil
}

func (httpsHandler *HTTPSHandler) SendClientHTTPSRequest() error {
	rawReq, err := httputil.DumpRequest(httpsHandler.clientRequest, true)
	_, err = httpsHandler.serverConn.Write(rawReq)
	if err != nil {
		return err
	}

	writer := bufio.NewReader(httpsHandler.serverConn)
	response, err := http.ReadResponse(writer, httpsHandler.clientRequest)
	if err != nil {
		return err
	}

	httpsHandler.response = response

	return nil
}

func (httpsHandler *HTTPSHandler) GetServerHTTPSResponse() error {
	rawResp, err := httputil.DumpResponse(httpsHandler.response, true)
	_, err = httpsHandler.clientConn.Write(rawResp)
	if err != nil {
		return err
	}

	return nil
}

func (h *RequestHandler) HandleHTTPRequest(w http.ResponseWriter, r *http.Request) {
	newReq, err := http.NewRequest(r.Method, r.URL.Path, r.Body)
	if err != nil {
		log.Println(err)
	}
	newReq.Host = r.Host
	newReq.URL.Scheme = "http"
	newReq.URL.Host = r.URL.Host
	newReq.Header = r.Header
	newReq.Header.Del("Proxy-Connection")
	client := http.Client{
		CheckRedirect: func(r *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	resp, err := client.Do(newReq)
	if err != nil {
		log.Println(err)
	}

	for header, values := range resp.Header {
		for _, value := range values {
			w.Header().Add(header, value)
		}
	}
	w.WriteHeader(resp.StatusCode)

	io.Copy(w, resp.Body)
	defer resp.Body.Close()
}

func main() {
	handler := &RequestHandler{
		router: mux.NewRouter(),
	}

	server := http.Server{
		Addr:    ":8080",
		Handler: http.HandlerFunc(handler.HandleRequest),
	}
	log.Println("server running at 8080")
	panic(server.ListenAndServe())
}
