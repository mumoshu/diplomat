package main

import (
	"context"
	"encoding/hex"
	"fmt"
	"github.com/gammazero/nexus/client"
	"github.com/gammazero/nexus/wamp"
	"log"
	"os"
	"os/signal"
	"sort"
	"time"
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
		realm:   "channel1",
		netAddr: "localhost",
		wsPort:  8000,
	}

	srvCloser, err := srv.ListenAndServe()
	if err != nil {
		log.Fatal(err)
	}
	defer srvCloser.Close()

	proc1 := "foo=bar"

	query1Callee, err := srv.localClient(proc1)
	if err != nil {
		log.Fatal(err)
	}

	handler := PrintBody(func(ctx context.Context, args wamp.List, kwargs wamp.Dict, details wamp.Dict) *client.InvokeResult {
		return sendData(ctx, query1Callee, gettysburg, args)
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

	if err = query1Callee.Register(proc1, handler, make(wamp.Dict)); err != nil {
		log.Fatalf("Failed to register %q: %s", proc1, err)
	}
	log.Printf("Registered procedure %q with router", proc1)

	// https://stackoverflow.com/questions/19992334/how-to-listen-to-n-channels-dynamic-select-statement

	caller, err := srv.localClient("CALLER")
	if err != nil {
		log.Fatal(err)
	}

	evt := []byte(`{"foo":{"id":1}}`)

	call(caller, proc1, evt, 64)

	topic1 := "topic1"
	// Subscribe to topic.
	subscriber, err := srv.localClient("subscriber")
	err = subscriber.Subscribe(topic1, createEvtHandler(topic1), nil)
	if err != nil {
		log.Fatal("subscribe error:", err)
	}
	log.Println("Subscribed to", topic1)

	intval := 1
	cond := RouteCondition{
		Expr{Path: []string{"foo", "id"}, Int: &intval},
	}
	srv.RouteToProcedure(cond, proc1)
	srv.RouteToTopic(cond, topic1)
	route := srv.GetRoute(cond)
	srv.Index(route)

	score, err := srv.Search(evt)
	if err != nil {
		panic(err)
	}

	fmt.Printf("score %+v", score)

	for id, v := range score {
		route := srv.GetRoute(id)
		thres := len(route.RouteCondition)
		if v < thres {
			log.Fatalf("skipping route %s due to low score: needs %d, got %d", route.ID(), thres, v)
		}
		topics := route.Topics
		procs := route.Procedures
		for _, t := range topics {
			if err := caller.Publish(t, nil, wamp.List{}, wamp.Dict{"body":evt}); err != nil {
				log.Fatal(err)
			}
		}

		if len(procs) > 1 {
			log.Fatalf("too many procs: %d", len(procs))
		}

		for _, p := range procs {
			res, err := call(caller, p, evt, 64)
			if err != nil {
				log.Fatal(err)
			}
			fmt.Printf("%+v", string(res))
		}
	}

	// Wait for SIGINT (CTRL-c), then close servers and exit.
	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, os.Interrupt)
	select {
	case <-shutdown:
	case <-query1Callee.Done():
		log.Print("query1Callee: Router gone, exiting")
		return // router gone, just exit
	case <-subscriber.Done():
		log.Print("subscriber: Router gone, exiting")
		return // router gone, just exit
	}

	if err = query1Callee.Unregister(proc1); err != nil {
		log.Println("Failed to unregister procedure:", err)
	}

	// Unsubscribe from topic.
	if err = subscriber.Unsubscribe(topic1); err != nil {
		log.Fatal("Failed to unsubscribe:", err)
	}

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
