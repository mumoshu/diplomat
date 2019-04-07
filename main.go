package main

import (
	"github.com/mumoshu/diplomat/pkg"
	"github.com/rs/xid"
	"log"
	"os"
	"os/signal"
)

func main() {
	realm := "channel1"
	netAddr := "0.0.0.0"
	wsPort := 8000
	srv := &diplomat.Server{
		RouteTable: &diplomat.RouteTable{
			RoutePartitions: map[uint64]*diplomat.RoutesPartition{},
		},
		RouteIndex: &diplomat.RouteIndex{
			Root: &diplomat.Node{
				Matchers: []*diplomat.Matcher{},
				Children: map[string]*diplomat.Node{},
			},
		},
		Realm:   realm,
		NetAddr: netAddr,
		WsPort:  wsPort,
	}

	srvCloser, err := srv.ListenAndServe()
	if err != nil {
		log.Fatal(err)
	}
	defer srvCloser.Close()

	intval := 1
	cond := diplomat.RouteCondition{
		diplomat.Expr{Path: []string{"foo", "id"}, Int: &intval},
	}
	proc1 := cond.Proc()
	topic1 := cond.Topic()
	srv.Register(cond, true, true)

	clientName := proc1
	localConn1, err := srv.Connect(clientName)
	if err != nil {
		log.Fatal(err)
	}

	srvDone, err := diplomat.Serve(localConn1, diplomat.When("foo", "id").EqInt(1), func(evt []byte) ([]byte, error) {
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
	err = subscriber.Subscribe(topic1, diplomat.CreateEvtHandler(sub1Id, topic1), nil)
	if err != nil {
		log.Fatal("subscribe error:", err)
	}
	log.Printf("%s subscribed to %s", sub1Id, topic1)

	sub2Id := "subscriber2-ws-" + xid.New().String()
	srvRef := diplomat.NewWsServerRef(realm, netAddr, wsPort)

	// WebSocket

	cond2 := diplomat.When("foo", "id").EqInt(2)

	srv.Register(cond2, true, false)

	wscli, err := srvRef.Connect(cond2.Proc())
	if err != nil {
		log.Fatalf("Connect failed: %v", err)
	}

	res, err := diplomat.ProgressiveCall(wscli, "/api/v1/register", map[string]interface{}{"proc": true}, 64)
	if err != nil {
		log.Fatalf("registration api call failed: %v", err)
	}
	log.Printf("res: %s", string(res))

	srv2Done, err := diplomat.Serve(wscli, cond2, func(evt []byte) ([]byte, error) {
		return evt, nil
	})
	if err != nil {
		log.Fatalf("Serve2 failed: %v", err)
	}

	subscriber2, err := srvRef.Connect(sub2Id)
	err = subscriber2.Subscribe(topic1, diplomat.CreateEvtHandler(sub2Id, topic1), nil)
	if err != nil {
		log.Fatal("subscribe error:", err)
	}
	log.Printf("%s subscribed to %s", sub2Id, topic1)

	res1, err := srv.ProcessEvent(evt)
	if err != nil {
		log.Fatalf("err: %v", err)
	}
	log.Printf("ProcessEvent returned: %s", string(res1))

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
