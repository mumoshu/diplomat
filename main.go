package main

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"github.com/mumoshu/diplomat/pkg"
	"github.com/mumoshu/diplomat/pkg/api"
	"log"
	"os"
	"os/signal"
)

func main() {
	realm := "channel1"
	netAddr := "0.0.0.0"
	wsPort := 8000
	srv := diplomat.NewServer(diplomat.Server{Realm: realm, NetAddr: netAddr, WsPort: wsPort})
	extHost := os.Getenv("EXT_HOST")

	srvCloser, err := srv.ListenAndServe()
	if err != nil {
		log.Fatal(err)
	}
	defer srvCloser.Close()

	echoWithFooIdEq1 := diplomat.On(api.ChannelEcho).Where("foo", "id").EqInt(1)
	echoSendChName := echoWithFooIdEq1.Channel.SendChannelURL()
	echoReceiveAllChName := echoWithFooIdEq1.Channel.SendChannelURL()

	// Echo Server

	localConn, err := srv.Connect("localConn")
	if err != nil {
		log.Fatal(err)
	}

	srvDone, err := localConn.ServeWithProgress(echoWithFooIdEq1, func(evt []byte) ([]byte, error) {
		return evt, nil
	})
	if err != nil {
		log.Fatalf("Serve failed: %v", err)
	}

	evt := []byte(`{"foo":{"id":1}}`)

	{
		err := srv.Publish(diplomat.Event{Channel: echoSendChName, Body: evt})
		if err != nil {
			log.Fatalf("TestProgressiveCall failed: %v", err)
		}
	}

	printingHandler := func(id string) func(evt interface{}) {
		return func(evt interface{}) {
			var r string
			b64, ok := evt.(string)
			// via websocket
			if ok {
				reader := base64.NewDecoder(base64.StdEncoding, bytes.NewBufferString(b64))
				buf := new(bytes.Buffer)
				_, err := buf.ReadFrom(reader)
				if err != nil {
					panic(err)
				}
				r = string(buf.Bytes())
			} else {
				// via local connection
				bytes, ok := evt.([]byte)
				if !ok {
					panic(fmt.Errorf("unexpected input: %v", evt))
				}
				r = string(bytes)
			}
			log.Printf("%s received: %v", id, r)
		}
	}

	// Subscribe to all the echo events
	receiveAllSub := "localEchoReceiveAllSubscriber"
	subConn, err := srv.Connect(receiveAllSub)
	if err != nil {
		log.Fatalf("%v", err)
	}
	err = subConn.SubscribeAny(diplomat.On(api.ChannelEcho).All(), printingHandler(receiveAllSub))
	if err != nil {
		log.Fatal("subscribe error:", err)
	}
	log.Printf("%s subscribed to %s", receiveAllSub, echoReceiveAllChName)

	// Subscribe to all the echo events

	receiveFooIdEq1Sub := "localEchoReceiveFooBarEq1Subscriber"
	sub2Conn, err := srv.Connect(receiveFooIdEq1Sub)
	err = sub2Conn.SubscribeAny(echoWithFooIdEq1, printingHandler(receiveFooIdEq1Sub))
	if err != nil {
		log.Fatal("subscribe error:", err)
	}
	log.Printf("%s subscribed to %s", receiveFooIdEq1Sub,  echoWithFooIdEq1.ReceiverName())

	// WebSocket

	srvRef := diplomat.NewWsServerRef(realm, netAddr, wsPort)

	echoWithFooIdEq2 := diplomat.On(api.ChannelEcho).Where("foo", "id").EqInt(2)
	wsConn, err := srvRef.Connect(echoWithFooIdEq2.Channel.SendChannelURL() + "Conn")
	if err != nil {
		log.Fatalf("Connect failed: %v", err)
	}

	if err := wsConn.ServeAny(echoWithFooIdEq2, func(in interface{}) (interface{}, error) {
		return in, nil
	}); err != nil {
		log.Fatalf("serve failed: %v", err)
	}

	sub2Id := "githubWebhookHandler"
	subscriber2, err := srvRef.Connect(sub2Id)
	cond3 := diplomat.OnURL(fmt.Sprintf("http://%s/webhook/github", extHost)).All()
	err = subscriber2.SubscribeAny(cond3, printingHandler(sub2Id))
	if err != nil {
		log.Fatal("subscribe error:", err)
	}
	log.Printf("%s subscribed to %s", sub2Id, cond3.ReceiverName())

	sub3Id := "slackWebhookHandler"
	cond4 := diplomat.OnURL(fmt.Sprintf("http://%s/webhook/slack", extHost)).All()
	err = subscriber2.SubscribeAny(cond4, printingHandler(sub3Id))
	if err != nil {
		log.Fatal("subscribe error:", err)
	}
	log.Printf("%s subscribed to %s", sub3Id, cond4.ReceiverName())

	// Register the webhook url at https://api.slack.com/apps/<ID>/interactive-messages?
	sub4Id := "slackInteractiveComponentsWebhookHandler"
	slackIntUrl := fmt.Sprintf("http://%s/webhook/slack-interactive", extHost)
	cond5 := diplomat.OnURL(slackIntUrl).All()
	//slackBotToken := os.Getenv("BOT_USER_OAUTH_ACCESS_TOKEN")
	slackVerificationToken := os.Getenv("VERIFICATION_TOKEN")
	slackInteractionsHandler := diplomat.SlackInteractionsHttpHandler{
		// verification token can be obtained at https://api.slack.com/apps/<APP ID>/general?
		// after creating an slack app
		VerificationToken: slackVerificationToken,
		CallbackID: "mycallback1",
	}
	cond5.FormParameterName = "payload"
	err = subscriber2.ServeHTTP(slackIntUrl, cond5, slackInteractionsHandler)
	if err != nil {
		log.Fatal("subscribe error:", err)
	}
	log.Printf("%s subscribed to %s", sub4Id, cond5.ReceiverName())

	// Wait for SIGINT (CTRL-c), then close servers and exit.
	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, os.Interrupt)
	select {
	case <-shutdown:
	case <-srvDone:
		log.Print("locallCalee: Router gone, exiting")
		return // router gone, just exit
	case <-subConn.Done():
		log.Print("subscriber: Router gone, exiting")
		return // router gone, just exit
	}

	if err = subConn.Unsubscribe(echoReceiveAllChName); err != nil {
		log.Fatal("Failed to unsubscribe:", err)
	}

	log.Fatal("Server is existing cleanly")

	// Servers close at exit due to defer calls.
}
