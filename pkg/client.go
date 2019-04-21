package diplomat

import (
	"bytes"
	"context"
	"fmt"
	"github.com/gammazero/nexus/client"
	"github.com/gammazero/nexus/wamp"
	"github.com/mumoshu/diplomat/pkg/api"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"time"
)

type CondBuilder struct {
	Channel api.ChannelRef
	Path    []string
}

func On(ch api.ChannelRef) CondBuilder {
	return CondBuilder{
		Channel: ch,
	}
}

func OnURL(u string) CondBuilder {
	parsed, err := url.Parse(u)
	if err != nil {
		panic(err)
	}
	return CondBuilder{
		Channel: api.ChannelRef{
			Scheme:      api.Scheme(parsed.Scheme),
			ChannelName: fmt.Sprintf("%s%s", parsed.Host, parsed.Path),
		},
	}
}

func (b CondBuilder) All() RouteCondition {
	c := RouteCondition{
		Channel: b.Channel,
	}
	c.Expressions = []Expr{{Path: b.Path, All: true}}
	return c
}

func (b CondBuilder) Where(path ...string) CondBuilder {
	b.Path = path
	return b
}

func (b CondBuilder) EqInt(v int) RouteCondition {
	c := RouteCondition{
		Channel: b.Channel,
	}
	if b.Path != nil {
		c.Expressions = []Expr{{Path: b.Path, Int: &v}}
	} else {
		c.Expressions = []Expr{}
	}
	return c
}

func (b CondBuilder) EqString(s string) RouteCondition {
	c := RouteCondition{
		Channel: b.Channel,
	}
	if b.Path != nil {
		c.Expressions = []Expr{{Path: b.Path, String: &s}}
	} else {
		c.Expressions = []Expr{}
	}
	return c
}

type Client struct {
	*client.Client
}

//func Serve(srv *Server, cond RouteCondition, func(evt []byte) ([]byte, error) {
//	topic1 := srv.AddConditionalRouteToTopic(cond)
//
//}
//

func (c *Client) Register(reg Registration) error {
	fmt.Printf("client: registering %v\n", reg)
	_, err := Call(c.Client, api.DiplomatRegisterChan.SendChannelURL(), reg)
	if err != nil {
		return fmt.Errorf("registration failed: %v", err)
	}
	return nil
}

func (c *Client) Serve(cond RouteCondition, f func(in interface{}) (interface{}, error)) error {
	if err := c.Register(Registration{RouteCondition: cond, Proc: true, Topic: false}); err != nil {
		return fmt.Errorf("registration failed: %v", err)
	}
	return c.serve(cond, f)
}

func (c *Client) serve(cond RouteCondition, f func(in interface{}) (interface{}, error)) error {
	cli := c.Client
	handler := c.FuncProcHandler(f)
	ch := cond.Channel
	proc := cond.ReceiverName()
	if err := cli.Register(proc, handler, make(wamp.Dict)); err != nil {
		return fmt.Errorf("Failed to register %q: %s", ch, err)
	}

	log.Printf("Registered procedure %s for channel %s with router", proc, ch)
	return nil
}

func (c *Client) FuncProcHandler(f func(in interface{}) (interface{}, error)) func(context.Context, wamp.List, wamp.Dict, wamp.Dict) *client.InvokeResult {
	return func(ctx context.Context, args wamp.List, kwargs wamp.Dict, details wamp.Dict) *client.InvokeResult {
		req := args[0]
		res, err := f(req)
		if err != nil {
			return &client.InvokeResult{Err: wamp.ErrInvalidArgument, Kwargs: wamp.Dict{"message": fmt.Sprintf("unexpected error: %v", err)}}
		}
		return &client.InvokeResult{Args: wamp.List{res}}
	}
}

func (c *Client) FuncSubHandler(f func(in interface{})) client.EventHandler {
	return func(args wamp.List, kwargs wamp.Dict, details wamp.Dict) {
		req := kwargs["body"]
		f(req)
	}
}

func (c *Client) ServeWithProgress(cond RouteCondition, f func(evt []byte) ([]byte, error)) (<-chan struct{}, error) {
	reg := Registration{RouteCondition: cond, Proc: true, Topic: false,}
	if err := c.Register(reg); err != nil {
		log.Fatalf("registration failed 1: %v", err)
	}

	return c.listenAndServeWithProgress(cond, f)
}

func (c *Client) Subscribe(cond RouteCondition, f func(evt interface{})) error {
	reg := Registration{RouteCondition: cond, Proc: false, Topic: true,}
	if err := c.Register(reg); err != nil {
		log.Fatalf("subscription registration failed : %v", err)
	}

	return c.subscribe(cond, f)
}

func (c *Client) subscribe(cond RouteCondition, f func(evt interface{})) error {
	err := c.Client.Subscribe(cond.ReceiverName(), c.FuncSubHandler(f), nil)
	if err != nil {
		return fmt.Errorf("subscription failed:", err)
	}
	log.Printf("%s subscribed to %s", c.ID(), cond.ReceiverName())
	return nil
}

type HttpHandler interface {
	ServeHTTP(w http.ResponseWriter, r *http.Request)
}

func (c *Client) SubscribeHTTP(url string, cond RouteCondition, handler HttpHandler) error {
	reg := Registration{RouteCondition: cond, Proc: false, Topic: true,}
	if err := c.Register(reg); err != nil {
		log.Fatalf("slack subscription registration failed : %v", err)
	}

	return c.subscribeHttp(url, cond, handler)
}

func (c *Client) ServeHTTP(url string, cond RouteCondition, handler HttpHandler) error {
	reg := Registration{RouteCondition: cond, Proc: true, Topic: false}
	if err := c.Register(reg); err != nil {
		return fmt.Errorf("slack serve registration failed: %v", err)
	}
	return c.serveHttp(url, cond, handler)
}

func (c *Client) serveHttp(uu string, cond RouteCondition, f http.Handler) error {
	cli := c.Client
	handler, err := httpInvocationHandlerAdapter(uu, f)
	if err != nil {
		return err
	}
	ch := cond.Channel
	proc := cond.ReceiverName()
	if err := cli.Register(proc, handler, make(wamp.Dict)); err != nil {
		return fmt.Errorf("Failed to register %q: %s", ch, err)
	}

	log.Printf("Registered procedure %s for channel %s with router", proc, ch)
	return nil
}


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

func (c *Client) subscribeHttp(uu string, cond RouteCondition, f HttpHandler) error {
	var handler client.EventHandler

	handler, err := httpHandlerAdapter(uu, f, nil)
	if err != nil {
		return err
	}
	err = c.Client.Subscribe(cond.ReceiverName(), handler, nil)
	if err != nil {
		return fmt.Errorf("subscription failed:", err)
	}
	log.Printf("%s subscribed to %s", c.ID(), cond.ReceiverName())
	return nil
}

func (c *Client) listenAndServeWithProgress(cond RouteCondition, f func(evt []byte) ([]byte, error)) (<-chan struct{}, error) {
	//proc1 := srv.AddConditionalRouteToProcedure(cond)

	procName := cond.ReceiverName()

	cli := c.Client

	//call(locallCalee, "AddConditionalRouteToProcedure", )

	localCalleeHandler := PrintBody(func(ctx context.Context, args wamp.List, kwargs wamp.Dict, details wamp.Dict) *client.InvokeResult {
		bs, err := getBodyBytes(kwargs)
		if err != nil {
			log.Fatalf("Unexpected error: %v", err)
		}
		if err != nil {
			return &client.InvokeResult{Err: wamp.ErrInvalidArgument, Kwargs: wamp.Dict{"message": fmt.Sprintf("Unexpected type: %T: %v", kwargs["body"], kwargs["body"])}}
		}
		data, err := f(bs)
		if err != nil {
			return &client.InvokeResult{Err: wamp.ErrInvalidArgument, Kwargs: wamp.Dict{"message": err.Error()}}
		}
		return progressiveSend(ctx, cli, data, args)
	})

	_ = func(ctx context.Context, args wamp.List, kwargs wamp.Dict, details wamp.Dict) *client.InvokeResult {
		now := time.Now()
		results := wamp.List{fmt.Sprintf("UTC: %s", now.UTC())}

		for i := range args {
			locName, ok := wamp.AsString(args[i])
			if !ok {
				continue
			}
			loc, err := time.LoadLocation(locName)
			if err != nil {
				results = append(results, fmt.Sprintf("%s: %s", locName, err))
				continue
			}
			results = append(results, fmt.Sprintf("%s: %s", locName, now.In(loc)))
		}

		return &client.InvokeResult{Args: results}
	}

	if err := cli.Register(procName, localCalleeHandler, make(wamp.Dict)); err != nil {
		return nil, fmt.Errorf("Failed to register %q: %s", procName, err)
	}

	log.Printf("Registered procedure %q with router", procName)
	return cli.Done(), nil
}
