package diplomat

import (
	"bytes"
	"context"
	"fmt"
	"github.com/gammazero/nexus/client"
	"github.com/gammazero/nexus/wamp"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
)

type ResponseWriter struct {
	Response map[string]interface{}
}

func (w *ResponseWriter) Header() http.Header {
	if w.Response == nil {
		w.Response = map[string]interface{}{}
	}
	_, ok := w.Response["header"]
	if !ok {
		w.Response["header"] = http.Header{}
	}
	return w.Response["header"].(http.Header)
}

func (w *ResponseWriter) Write(body []byte) (int, error) {
	w.Response["body"] = body
	return len(body), nil
}

func (w *ResponseWriter) WriteHeader(statusCode int) {
	if w.Response == nil {
		w.Response = map[string]interface{}{}
	}
	w.Response["statusCode"] = statusCode
}

func httpHandlerAdapter(uu string, f HttpHandler, onResponse func(map[string]interface{}) error) (func(args wamp.List, kwargs wamp.Dict, details wamp.Dict), error) {
	u, err := url.Parse(uu)
	if err != nil {
		return nil, err
	}
	handler := func(args wamp.List, kwargs wamp.Dict, details wamp.Dict) {
		r, err := wampMessageToHttpRequest(u, args, kwargs, details)
		if err != nil {
			log.Printf("unable to obtain request body: %v", err)
			return
		}
		resWriter := &ResponseWriter{}
		f.ServeHTTP(resWriter, r)
		if onResponse != nil {
			err := onResponse(resWriter.Response)
			if err != nil {
				fmt.Fprintf(os.Stderr, "on response error: %v", err)
			}
		} else {
			resBody := resWriter.Response["body"]
			if resBody != nil {
				fmt.Fprintf(os.Stderr, "discarding response body because  no response handler provided. perhaps this is an subscription?")
			}
		}
	}
	return handler, nil
}

func wampMessageToHttpRequest(u *url.URL, args wamp.List, kwargs wamp.Dict, details wamp.Dict) (*http.Request, error) {
	body, err := getBodyBytes(kwargs)
	if err != nil {
		return nil, err
	}
	bodyReader := ioutil.NopCloser(bytes.NewReader(body))
	r := &http.Request{
		Method:     http.MethodPost,
		URL:        u,
		Proto:      "http",
		ProtoMajor: 1,
		ProtoMinor: 1,
		Header:     getHttpHeader(kwargs),
		Body:       bodyReader,
		GetBody: func() (io.ReadCloser, error) {
			return bodyReader, nil
		},
		ContentLength:    int64(len(body)),
		TransferEncoding: []string{},
	}
	return r, nil
}

func httpInvocationHandlerAdapter(uu string, f HttpHandler) (func(ctx context.Context, args wamp.List, kwargs wamp.Dict, details wamp.Dict) *client.InvokeResult, error) {
	u, err := url.Parse(uu)
	if err != nil {
		return nil, err
	}
	handler := func(ctx context.Context, args wamp.List, kwargs wamp.Dict, details wamp.Dict) *client.InvokeResult {
		r, err := wampMessageToHttpRequest(u, args, kwargs, details)
		if err != nil {
			return &client.InvokeResult{Err: wamp.ErrInvalidArgument, Kwargs: wamp.Dict{"message": fmt.Sprintf("unexpected error: %v", err)}}
		}
		resWriter := &ResponseWriter{}
		f.ServeHTTP(resWriter, r)
		return &client.InvokeResult{Kwargs: wamp.Dict(resWriter.Response)}
	}
	return handler, nil
}
