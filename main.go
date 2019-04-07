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
	proc1 := srv.AddConditionalRouteToProcedure(cond)
	topic1 := srv.AddConditionalRouteToTopic(cond)

	srvDone, err := Serve(srv, When("foo", "id").EqInt(1), func(evt []byte) ([]byte, error) {
		return evt, nil
	})
	if err != nil {
		log.Fatalf("Serve failed: %v", err)
	}

	//locallCalee, err := srv.Connect(proc1)
	//if err != nil {
	//	log.Fatal(err)
	//}
	//localCalleeHandler := PrintBody(func(ctx context.Context, args wamp.List, kwargs wamp.Dict, details wamp.Dict) *client.InvokeResult {
	//	return progressiveSend(ctx, locallCalee, gettysburg, args)
	//})
	//
	//_ = func(ctx context.Context, args wamp.List, kwargs wamp.Dict, details wamp.Dict) *client.InvokeResult {
	//	now := time.Now()
	//	results := wamp.List{fmt.Sprintf("UTC: %s", now.UTC())}
	//
	//	for i := range args {
	//		locName, ok := wamp.AsString(args[i])
	//		if !ok {
	//			continue
	//		}
	//		loc, err := time.LoadLocation(locName)
	//		if err != nil {
	//			results = append(results, fmt.Sprintf("%s: %s", locName, err))
	//			continue
	//		}
	//		results = append(results, fmt.Sprintf("%s: %s", locName, now.In(loc)))
	//	}
	//
	//	return &client.InvokeResult{Args: results}
	//}
	//
	//if err = locallCalee.Register(proc1, localCalleeHandler, make(wamp.Dict)); err != nil {
	//	log.Fatalf("Failed to register %q: %s", proc1, err)
	//}
	//log.Printf("Registered procedure %q with router", proc1)

	// https://stackoverflow.com/questions/19992334/how-to-listen-to-n-channels-dynamic-select-statement

	caller, err := srv.Connect("CALLER")
	if err != nil {
		log.Fatal(err)
	}

	srv.caller = caller

	evt := []byte(`{"foo":{"id":1}}`)

	progressiveCall(caller, proc1, evt, 64)

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

	intval2 := 2
	cond2 := RouteCondition{
		Expr{Path: []string{"foo", "id"}, Int: &intval2},
	}
	srv.AddConditionalRouteToProcedure(cond2)
	log.Printf("Route added: %v", cond2)
	route2 := srv.GetRoute(cond2)
	srv.Index(route2)

	srv2Done, err := Serve(srvRef, When("foo", "id").EqInt(2), func(evt []byte) ([]byte, error) {
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

	route := srv.GetRoute(cond)
	srv.Index(route)

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
