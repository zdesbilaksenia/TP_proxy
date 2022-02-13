package main

import (
	"github.com/gorilla/mux"
	"io"
	"log"
	"net/http"
)

type HandlerHttp struct {
	router *mux.Router
}

func (h *HandlerHttp) HandleHTTPRequest(w http.ResponseWriter, r *http.Request) {
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
	handler := &HandlerHttp{
		router: mux.NewRouter(),
	}

	server := http.Server{
		Addr:    ":8080",
		Handler: http.HandlerFunc(handler.HandleHTTPRequest),
	}
	log.Println("server running at 8080")
	panic(server.ListenAndServe())
}
