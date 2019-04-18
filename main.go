package main

import (
	"fmt"
	"github.com/mumoshu/diplomat/pkg"
	"github.com/mumoshu/diplomat/pkg/api"
	"github.com/rs/xid"
	"log"
	"os"
	"os/signal"
	"time"
	"github.com/mitchellh/mapstructure"
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

	echoWithFooIdEq1 := diplomat.On(api.DiplomatEchoChan).Where("foo", "id").EqInt(1)
	echoSendChName := echoWithFooIdEq1.Channel.SendChannelURL()
	echoFooBar1Topic := echoWithFooIdEq1.Channel.SendChannelURL()
	//srv.Register(cond, true, true)

	clientName := echoSendChName + "Server"
	localRegistrationServerConn, err := srv.Connect(clientName)
	if err != nil {
		log.Fatal(err)
	}

	// Registration Server

	ResponseOK := "OK"
	if err := localRegistrationServerConn.Serve(diplomat.On(api.DiplomatRegisterChan).All(), func(in interface{}) (interface{}, error) {
		var reg diplomat.Registration
		var ok bool
		reg, ok = in.(diplomat.Registration)
		if !ok {
			if err := mapstructure.Decode(in, &reg); err != nil {
				return nil, fmt.Errorf("registration server: unexpected type of input %T: %v: %v", in, in, err)
			}
		}
		srv.Register(reg)
		return ResponseOK, err
	}); err != nil {
		log.Fatal(err)
	}

	time.Sleep(1 * time.Second)

	localConn, err := srv.Connect("localConn")
	if err != nil {
		log.Fatal(err)
	}
	reg := diplomat.Registration{RouteCondition: echoWithFooIdEq1, Proc: true, Topic: true,}
	if err := localConn.Register(reg); err != nil {
		log.Fatalf("registration failed 1: %v", err)
	}

	// Echo Server

	srvDone, err := localRegistrationServerConn.ListenAndServeWithProgress(echoWithFooIdEq1, func(evt []byte) ([]byte, error) {
		return evt, nil
	})
	if err != nil {
		log.Fatalf("Serve failed: %v", err)
	}

	evt := []byte(`{"foo":{"id":1}}`)

	{
		_, err := srv.ProcessEvent(echoSendChName, evt)
		if err != nil {
			log.Fatalf("TestProgressiveCall failed: %v", err)
		}
	}

	// Subscribe to topic.
	sub1Id := "subscriber-local-" + xid.New().String()
	subscriber, err := srv.Connect(sub1Id)
	err = subscriber.Subscribe(echoFooBar1Topic, diplomat.CreateEvtHandler(sub1Id, echoFooBar1Topic), nil)
	if err != nil {
		log.Fatal("subscribe error:", err)
	}
	log.Printf("%s subscribed to %s", sub1Id, echoFooBar1Topic)

	sub2Id := "subscriber2-ws-" + xid.New().String()
	srvRef := diplomat.NewWsServerRef(realm, netAddr, wsPort)

	// WebSocket

	echoWithFooIdEq2 := diplomat.On(api.DiplomatEchoChan).Where("foo", "id").EqInt(2)

	wscli, err := srvRef.Connect(echoWithFooIdEq2.Channel.SendChannelURL() + "Conn")
	if err != nil {
		log.Fatalf("Connect failed: %v", err)
	}

	if err := wscli.Register(diplomat.Registration{RouteCondition: echoWithFooIdEq2, Proc: true, Topic: false}); err != nil {
		log.Fatalf("registration failed: %v", err)
	}

	if err := wscli.Serve(echoWithFooIdEq2, func(in interface{}) (interface{}, error) {
		return in, nil
	}); err != nil {
		log.Fatalf("serve failed: %v", err)
	}

	subscriber2, err := srvRef.Connect(sub2Id)
	err = subscriber2.Subscribe(echoFooBar1Topic, diplomat.CreateEvtHandler(sub2Id, echoFooBar1Topic), nil)
	if err != nil {
		log.Fatal("subscribe error:", err)
	}
	log.Printf("%s subscribed to %s", sub2Id, echoFooBar1Topic)

	res1, err := srv.ProcessEvent(api.DiplomatEchoChan.SendChannelURL(), evt)
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
	//case <-srv2Done:
	//	log.Print("locallCalee: Router2 gone, exiting")
	//	return // router gone, just exit
	case <-subscriber.Done():
		log.Print("subscriber: Router gone, exiting")
		return // router gone, just exit
	}

	//if err = locallCalee.Unregister(proc1); err != nil {
	//	log.Println("Failed to unregister procedure:", err)
	//}

	// Unsubscribe from topic.
	if err = subscriber.Unsubscribe(echoFooBar1Topic); err != nil {
		log.Fatal("Failed to unsubscribe:", err)
	}

	log.Fatal("Server is existing cleanly")

	// Servers close at exit due to defer calls.
}
