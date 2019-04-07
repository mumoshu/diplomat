package diplomat

import (
	"bytes"
	"fmt"
	"github.com/gammazero/nexus/client"
	"github.com/gammazero/nexus/router"
	"github.com/gammazero/nexus/wamp"
	"io"
	"log"
	"net/http"
	"os"
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

	Realm    string
	NetAddr  string
	WsPort   int
	HttpPort int

	nxr router.Router

	internalClient *client.Client
}

func (s *Server) ListenAndServe() (io.Closer, error) {
	var (
		netAddr  = s.NetAddr
		wsPort   = s.WsPort
		httpPort = s.HttpPort
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
				URI:           wamp.URI(s.Realm),
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

	localCallerConn, err := s.Connect("LOCAL_CLIENT")
	if err != nil {
		return nil, err
	}

	s.internalClient = localCallerConn

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
		Realm:  s.Realm,
		Logger: logger,
	}
	c, err := client.ConnectLocal(s.nxr, cfg)
	if err != nil {
		return nil, err
	}

	return c, nil
}

func (srv *Server) Register(cond RouteCondition, proc, topic bool) {
	if proc {
		srv.AddConditionalRouteToProcedure(cond)
	}
	if topic {
		srv.AddConditionalRouteToTopic(cond)
	}
	log.Printf("Route added: %v", cond)
	route := srv.GetRoute(cond)
	srv.Index(route)
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
			if err := srv.internalClient.Publish(t, nil, wamp.List{}, wamp.Dict{"body": evt}); err != nil {
				log.Fatal(err)
			}
		}

		if len(procs) > 1 {
			log.Fatalf("too many procs: %d", len(procs))
		}

		for _, p := range procs {
			return ProgressiveCall(srv.internalClient, p, evt, 64)
		}
	}
	return []byte(`{"message":"no handler found"}`), nil
}

func (srv *Server) TestCall(procName string, evt []byte) ([]byte, error) {
	return ProgressiveCall(srv.internalClient, procName, evt, 64)
}
