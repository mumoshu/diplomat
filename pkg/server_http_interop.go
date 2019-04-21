package diplomat

import (
	"bytes"
	"fmt"
	"log"
	"net/http"
	"strings"
)

func (srv *Server) CreateHttpHandler() func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		bufbody := new(bytes.Buffer)
		_, err := bufbody.ReadFrom(r.Body)
		if err != nil {
			log.Fatalf("unable to read body: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		httpReqBody := bufbody.Bytes()
		if strings.Index(r.URL.Path, "/") != 0 {
			log.Printf("http handler failed: invalid path: path should start with /: %s", r.URL.Path)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		header := map[string][]string(r.Header)
		url := "http://" + r.Host + r.URL.Path
		log.Printf("processing request to %s", url)
		res, err := srv.Call(Event{Channel: url, Body: httpReqBody, Header: header})
		if err != nil {
			log.Printf("http handler failed: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		log.Printf("call finished. response: header=%v body=%s", res.Header, string(res.Body))

		for k, vs := range res.Header {
			for _, v := range vs {
				w.Header().Add(k, v)
			}
		}
		if res.StatusCode != 0 {
			w.WriteHeader(res.StatusCode)
		}
		_, err = w.Write(res.Body)
		if err != nil {
			panic(fmt.Errorf("unable to write: %v", err))
		}
	}
}

