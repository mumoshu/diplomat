package diplomat

import (
	"bytes"
	"fmt"
	"github.com/gammazero/nexus/client"
	"github.com/gammazero/nexus/router"
	"github.com/gammazero/nexus/wamp"
	"github.com/mitchellh/mapstructure"
	"github.com/mumoshu/diplomat/pkg/api"
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

	Realm    string
	NetAddr  string
	WsPort   int
	HttpPort int

	nxr router.Router

	internalClient *Client
}

func NewServer(opts Server) *Server {
	return &Server{
		RouteTable: &RouteTable{
			RoutePartitions: map[uint64]*RoutesPartition{},
		},
		RouteIndex: &RouteIndex{
		},
		Realm:   opts.Realm,
		NetAddr: opts.NetAddr,
		WsPort:  opts.WsPort,
	}
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

	if err := s.startRegistrationServer(); err != nil {
		return nil, err
	}

	return wsCloser, nil
}

type ServerRef interface {
	Connect(name string) (*client.Client, error)
}

type RemoteServerRef struct {
	Realm string
	URL   string
}

type Registration struct {
	RouteCondition `mapstructure:",squash"`
	Proc  bool
	Topic bool
}

func (s *Server) startRegistrationServer() error {
	clientName := "diplomatRegistrationServer"
	localRegistrationServerConn, err := s.Connect(clientName)
	if err != nil {
		log.Fatal(err)
	}

	// Registration Server

	ResponseOK := "OK"
	if err := localRegistrationServerConn.serve(On(api.DiplomatRegisterChan).All(), func(in interface{}) (interface{}, error) {
		var reg Registration
		var ok bool
		reg, ok = in.(Registration)
		if !ok {
			fmt.Printf("decoding %v", in)
			config := &mapstructure.DecoderConfig{
				ErrorUnused: true,
				Metadata: nil,
				Result:   &reg,
			}
			decoder, err := mapstructure.NewDecoder(config)
			if err != nil {
				return nil, err
			}
			if err := decoder.Decode(in); err != nil {
				return nil, fmt.Errorf("registration server: unexpected type of input %T: %v: %v", in, in, err)
			}
		}
		fmt.Printf("server: registering %v\n", reg)
		s.Register(reg)
		return ResponseOK, err
	}); err != nil {
		return err
	}
	return nil
}

func (s *Server) Connect(name string) (*Client, error) {
	logger := log.New(os.Stdout, fmt.Sprintf("local %s> ", name), log.LstdFlags)
	cfg := client.Config{
		Realm:  s.Realm,
		Logger: logger,
	}
	c, err := client.ConnectLocal(s.nxr, cfg)
	if err != nil {
		return nil, err
	}

	return &Client{c}, nil
}

func (srv *Server) Register(reg Registration) {
	if reg.Proc {
		srv.AddConditionalRouteToProcedure(reg.RouteCondition)
	}
	if reg.Topic {
		srv.AddConditionalRouteToTopic(reg.RouteCondition)
	}
	log.Printf("Route added: %v", reg.RouteCondition)
	route := srv.GetRoute(reg.RouteCondition)
	srv.Index(route)
}

func NewWsServerRef(realm, host string, port int) *RemoteServerRef {
	return &RemoteServerRef{
		Realm: realm,
		URL:   fmt.Sprintf("ws://%s:%d", host, port),
	}
}

func (s *RemoteServerRef) Connect(name string) (*Client, error) {
	logger := log.New(os.Stdout, fmt.Sprintf("ws %s> ", name), log.LstdFlags)
	cfg := client.Config{
		Realm:  s.Realm,
		Logger: logger,
	}
	c, err := client.ConnectNet(s.URL, cfg)
	if err != nil {
		return nil, err
	}

	return &Client{c}, nil
}

func (srv *Server) CreateHttpHandler() func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		bufbody := new(bytes.Buffer)
		_, err := bufbody.ReadFrom(r.Body)
		if err != nil {
			log.Fatalf("unable to read body: %v", err)
		}
		evt := bufbody.Bytes()
		if strings.Index(r.URL.Path, "/") != 0 {
			log.Printf("http handler failed: invalid path: path should start with /: %s", r.URL.Path)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		url := "http://" + r.Host + r.URL.Path
		log.Printf("processing request to %s", url)
		res, err := srv.ProcessEvent(url, evt)
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

func (srv *Server) ProcessEvent(sendproc string, evt2 interface{}) ([]byte, error) {
	log.Printf("Processing event: %v", evt2)

	if err := srv.internalClient.Publish(sendproc, nil, wamp.List{}, wamp.Dict{"body": evt2}); err != nil {
		log.Fatal(err)
	}

	var evt []byte

	evt, ok := evt2.([]byte)
	if !ok {
		log.Printf("skipping %v because it isn't JSON", evt2)
		return nil, nil
	}

	idsAndScores, err := srv.SearchRouteMatchesChannelAndJSON(sendproc, evt)
	if err != nil {
		return nil, fmt.Errorf("handle event failed: %v", err)
	}

	fmt.Printf("score %+v\n", idsAndScores)

	procHandled := false

	for routeCondId, score := range idsAndScores {
		route := srv.GetRoute(routeCondId)
		thres := len(route.RouteCondition.Expressions)
		if score < thres {
			log.Fatalf("skipping route %s due to low score: needs %d, got %d", route.ID(), thres, score)
		}
		topics := route.Topics
		procs := route.Procedures
		fmt.Printf("publishing to %s\n", topics)
		for _, t := range topics {
			if err := srv.internalClient.Publish(t, nil, wamp.List{}, wamp.Dict{"body": evt}); err != nil {
				log.Fatal(err)
			}
		}

		if len(procs) > 1 {
			log.Fatalf("too many procs: %d", len(procs))
		}

		for _, p := range procs {
			_, err := ProgressiveCall(srv.internalClient.Client, p, evt, 64)
			if err != nil {
				log.Fatal(err)
			} else {
				procHandled = true
			}
		}
	}
	if procHandled {
		return []byte(`{"message":"proc handled"`), nil
	}
	return []byte(`{"message":"no proc handler found"}`), nil
}

//func (srv *Server) TestProgressiveCall(procName string, evt []byte) ([]byte, error) {
//	return ProgressiveCall(srv.internalClient.Client, procName, evt, 64)
//}
