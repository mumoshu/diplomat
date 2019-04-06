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

	nxr router.Router
}

func (s *Server) ListenAndServe() (io.Closer, error) {
	var (
		netAddr = "localhost"
		wsPort  = 8000
	)

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

	return wsCloser, nil
}

func (s *Server) localClient(name string) (*client.Client, error) {
	logger := log.New(os.Stdout, fmt.Sprintf("%s> ", name), log.LstdFlags)
	cfg := client.Config{
		Realm:  s.realm,
		Logger: logger,
	}
	callee, err := client.ConnectLocal(s.nxr, cfg)
	if err != nil {
		return nil, err
	}

	return callee, nil
}

func call(caller *client.Client, procedureName string, body []byte, chunkSize int) ([]byte, error) {
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
		ctx, procedureName, nil, wamp.List{chunkSize}, wamp.Dict{"body": body}, "", progHandler)
	if err != nil {
		return nil, fmt.Errorf("Failed to call procedure:", err)
	}

	// As a final result, the callee returns the base64 encoded sha256 hash of
	// the data.  This is decoded and compared to the value that the caller
	// calculated.  If they match, then the caller recieved the data correctly.
	hashB64 := result.Arguments[0].(string)
	log.Println("received hashB64: %s", hashB64)
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
