package main

import (
	"encoding/hex"
	"fmt"
	"github.com/rs/xid"
	"log"
	"os"
	"os/signal"
	"sort"
)

var key []byte

func init() {
	var err error
	key, err = hex.DecodeString("000102030405060708090A0B0C0D0E0FF0E0D0C0B0A090807060504030201000") // use your own key here
	if err != nil {
		fmt.Printf("Cannot decode hex key: %v", err) // add error handling
		return
	}
}

func main() {
	realm := "channel1"
	netAddr := "0.0.0.0"
	wsPort := 8000
	srv := &Server{
		RouteTable: &RouteTable{
			RoutePartitions: map[uint64]*RoutesPartition{},
		},
		RouteIndex: &RouteIndex{
			root: &Node{
				Matchers: []*Matcher{},
				Children: map[string]*Node{},
			},
		},
		realm:   realm,
		netAddr: netAddr,
		wsPort:  wsPort,
	}

	srvCloser, err := srv.ListenAndServe()
	if err != nil {
		log.Fatal(err)
	}
	defer srvCloser.Close()

	intval := 1
	cond := RouteCondition{
		Expr{Path: []string{"foo", "id"}, Int: &intval},
	}
	proc1 := cond.Proc()
	topic1 := cond.Topic()
	srv.Register(cond, true, true)

	clientName := proc1
	localConn1, err := srv.Connect(clientName)
	if err != nil {
		log.Fatal(err)
	}

	srvDone, err := Serve(localConn1, When("foo", "id").EqInt(1), func(evt []byte) ([]byte, error) {
		return evt, nil
	})
	if err != nil {
		log.Fatalf("Serve failed: %v", err)
	}

	evt := []byte(`{"foo":{"id":1}}`)

	srv.TestCall(proc1, evt)

	// Subscribe to topic.
	sub1Id := "subscriber-local-" + xid.New().String()
	subscriber, err := srv.Connect(sub1Id)
	err = subscriber.Subscribe(topic1, createEvtHandler(sub1Id, topic1), nil)
	if err != nil {
		log.Fatal("subscribe error:", err)
	}
	log.Printf("%s subscribed to %s", sub1Id, topic1)

	sub2Id := "subscriber2-ws-" + xid.New().String()
	srvRef := NewWsServerRef(realm, netAddr, wsPort)

	// WebSocket

	cond2 := When("foo", "id").EqInt(2)

	srv.Register(cond2, true, false)

	wscli, err := srvRef.Connect(cond2.Proc())
	if err != nil {
		log.Fatalf("Connect failed: %v", err)
	}

	srv2Done, err := Serve(wscli, cond2, func(evt []byte) ([]byte, error) {
		return evt, nil
	})
	if err != nil {
		log.Fatalf("Serve2 failed: %v", err)
	}

	subscriber2, err := srvRef.Connect(sub2Id)
	err = subscriber2.Subscribe(topic1, createEvtHandler(sub2Id, topic1), nil)
	if err != nil {
		log.Fatal("subscribe error:", err)
	}
	log.Printf("%s subscribed to %s", sub2Id, topic1)

	res, err := srv.ProcessEvent(evt)
	if err != nil {
		log.Fatalf("err: %v", err)
	}
	log.Printf("ProcessEvent returned: %s", string(res))

	// Wait for SIGINT (CTRL-c), then close servers and exit.
	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, os.Interrupt)
	select {
	case <-shutdown:
	case <-srvDone:
		log.Print("locallCalee: Router gone, exiting")
		return // router gone, just exit
	case <-srv2Done:
		log.Print("locallCalee: Router2 gone, exiting")
		return // router gone, just exit
	case <-subscriber.Done():
		log.Print("subscriber: Router gone, exiting")
		return // router gone, just exit
	}

	//if err = locallCalee.Unregister(proc1); err != nil {
	//	log.Println("Failed to unregister procedure:", err)
	//}

	// Unsubscribe from topic.
	if err = subscriber.Unsubscribe(topic1); err != nil {
		log.Fatal("Failed to unsubscribe:", err)
	}

	log.Fatal("Server is existing cleanly")

	// Servers close at exit due to defer calls.
}

type Query struct {
	Data map[string]string
}

func (q Query) String() string {
	keys := make([]string, len(q.Data))
	i := 0
	for k := range q.Data {
		keys[i] = k
		i ++
	}
	sort.Strings(keys)
	s := ""
	for i := range keys {
		if s != "" {
			s += " "
		}
		k := keys[i]
		s += "+" + k + ":" + q.Data[k]
	}
	return s
}
