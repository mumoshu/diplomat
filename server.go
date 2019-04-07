package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"github.com/gammazero/nexus/client"
	"github.com/gammazero/nexus/router"
	"github.com/gammazero/nexus/wamp"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"
)

type Closer struct {
	wsCloser io.Closer
	nxr      router.Router
}

func (c *Closer) Close() error {
	err1 := c.wsCloser.Close()
	c.nxr.Close()
	return err1
}

type Server struct {
	*RouteTable
	*RouteIndex

	realm   string
	netAddr string
	wsPort  int
	httpPort int

	nxr router.Router

	caller *client.Client
}

func (s *Server) ListenAndServe() (io.Closer, error) {
	var (
		netAddr  = s.netAddr
		wsPort   = s.wsPort
		httpPort = s.httpPort
	)

	if netAddr == "" {
		netAddr = "0.0.0.0"
	}
	if wsPort == 0 {
		wsPort = 8000
	}
	if httpPort == 0 {
		httpPort = 9001
	}

	routerConfig := &router.Config{
		RealmConfigs: []*router.RealmConfig{
			&router.RealmConfig{
				URI:           wamp.URI(s.realm),
				AnonymousAuth: true,
				AllowDisclose: true,
			},
		},
	}

	closer := &Closer{}

	nxr, err := router.NewRouter(routerConfig, nil)
	if err != nil {
		return nil, err
	}

	s.nxr = nxr

	closer.nxr = nxr

	// wss server
	// Create websocket server.
	wss := router.NewWebsocketServer(nxr)
	// Enable websocket compression, which is used if clients request it.
	wss.Upgrader.EnableCompression = true
	// Configure server to send and look for client tracking cookie.
	wss.EnableTrackingCookie = true
	// Set keep-alive period to 30 seconds.
	wss.KeepAlive = 30 * time.Second

	// ---- Start servers ----

	// Run websocket server.
	wsAddr := fmt.Sprintf("%s:%d", netAddr, wsPort)
	wsCloser, err := wss.ListenAndServe(wsAddr)
	if err != nil {
		return closer, err
	}
	closer.wsCloser = wsCloser

	log.Printf("Websocket server listening on ws://%s/", wsAddr)

	httpHandler := s.CreateHttpHandler()
	mux := http.NewServeMux()
	mux.HandleFunc("/", httpHandler)
	go func() {
		httpAddr := fmt.Sprintf("%s:%d", netAddr, httpPort)
		log.Printf("Http server listening on %s", httpAddr)
		err := http.ListenAndServe(httpAddr, mux)
		if err != nil {
			log.Fatalf("error: %v", err)
		}
	}()

	return wsCloser, nil
}

type ServerRef interface {
	Connect(name string) (*client.Client, error)
}

type RemoteServerRef struct {
	Realm string
	URL   string
}

func (s *Server) Connect(name string) (*client.Client, error) {
	logger := log.New(os.Stdout, fmt.Sprintf("local %s> ", name), log.LstdFlags)
	cfg := client.Config{
		Realm:  s.realm,
		Logger: logger,
	}
	c, err := client.ConnectLocal(s.nxr, cfg)
	if err != nil {
		return nil, err
	}

	return c, nil
}

func NewWsServerRef(realm, host string, port int) *RemoteServerRef {
	return &RemoteServerRef{
		Realm: realm,
		URL: fmt.Sprintf("ws://%s:%d", host, port),
	}
}

func (s *RemoteServerRef) Connect(name string) (*client.Client, error) {
	logger := log.New(os.Stdout, fmt.Sprintf("ws %s> ", name), log.LstdFlags)
	cfg := client.Config{
		Realm:  s.Realm,
		Logger: logger,
	}
	c, err := client.ConnectNet(s.URL, cfg)
	if err != nil {
		return nil, err
	}

	return c, nil
}

func progressiveCall(caller *client.Client, procedureName string, evt []byte, chunkSize int) ([]byte, error) {
	// The progress handler accumulates the chunks of data as they arrive.  It
	// also progressively calculates a sha256 hash of the data as it arrives.
	var chunks []string
	h := sha256.New()
	progHandler := func(result *wamp.Result) {
		// Received another chunk of data, computing hash as chunks received.
		chunk := result.Arguments[0].(string)
		log.Println("Received", len(chunk), "bytes (as progressive result)")
		log.Println(string(chunk))
		chunks = append(chunks, chunk)
		h.Write([]byte(chunk))
	}

	ctx := context.Background()

	// Call the example procedure, specifying the size of chunks to send as
	// progressive results.
	result, err := caller.CallProgress(
		ctx, procedureName, nil, wamp.List{chunkSize}, wamp.Dict{"body": evt}, "", progHandler)
	if err != nil {
		return nil, fmt.Errorf("Failed to call procedure:", err)
	}

	// As a final result, the callee returns the base64 encoded sha256 hash of
	// the data.  This is decoded and compared to the value that the caller
	// calculated.  If they match, then the caller recieved the data correctly.
	hashB64 := result.Arguments[0].(string)
	log.Printf("Received hashB64: %s", hashB64)
	calleeHash, err := base64.StdEncoding.DecodeString(hashB64)
	if err != nil {
		return nil, fmt.Errorf("decode error:", err)
	}

	// Check if received hash matches the hash computed over the received data.
	if !bytes.Equal(calleeHash, h.Sum(nil)) {
		return nil, fmt.Errorf("Hash of received data does not match")
	}
	log.Println("Correctly received all data:")
	log.Println("----------------------------")
	res := strings.Join(chunks, "")
	log.Println(res)

	return []byte(res), nil
}

func (srv *Server) CreateHttpHandler() func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		bufbody := new(bytes.Buffer)
		_, err := bufbody.ReadFrom(r.Body)
		if err != nil {
			log.Fatalf("unable to read body: %v", err)
		}
		evt := bufbody.Bytes()
		res, err := srv.ProcessEvent(evt)
		if err != nil {
			log.Printf("http handler failed: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		log.Printf("handled http request: response = %s", string(res))

		_, err = w.Write(res)
		if err != nil {
			panic(fmt.Errorf("unable to write: %v", err))
		}
	}
}

func (srv *Server) ProcessEvent(evt []byte) ([]byte, error) {
	log.Printf("Processing event: %v", string(evt))

	idsAndScores, err := srv.Search(evt)
	if err != nil {
		return nil, fmt.Errorf("handle event failed: %v", err)
	}

	fmt.Printf("score %+v\n", idsAndScores)

	for routeCondId, score := range idsAndScores {
		route := srv.GetRoute(routeCondId)
		thres := len(route.RouteCondition)
		if score < thres {
			log.Fatalf("skipping route %s due to low score: needs %d, got %d", route.ID(), thres, score)
		}
		topics := route.Topics
		procs := route.Procedures
		for _, t := range topics {
			if err := srv.caller.Publish(t, nil, wamp.List{}, wamp.Dict{"body": evt}); err != nil {
				log.Fatal(err)
			}
		}

		if len(procs) > 1 {
			log.Fatalf("too many procs: %d", len(procs))
		}

		for _, p := range procs {
			return progressiveCall(srv.caller, p, evt, 64)
		}
	}
	return []byte(`{"message":"no handler found"}`), nil
}
