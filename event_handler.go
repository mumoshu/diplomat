package main

import (
	"encoding/base64"
	"github.com/gammazero/nexus/wamp"
	"log"
)

func createEvtHandler(subId, topic string) func(args wamp.List, kwargs wamp.Dict, details wamp.Dict) {
	return func(args wamp.List, kwargs wamp.Dict, details wamp.Dict) {
		log.Println("Subscriber", subId, "received ", topic, "event")
		if len(args) != 0 {
			log.Println("  Event Arg[0]:", args[0])
		}
		if len(kwargs) != 0 {
			bs, ok := kwargs["body"].([]byte)
			if !ok {
				log.Printf("Decoding base64: %v", kwargs["body"])
				s, isStr := kwargs["body"].(string)
				if !isStr {
					log.Fatalf("Unexpected body: %T: %v", kwargs["body"], kwargs["body"])
					return
				}
				var err error
				bs, err = base64.StdEncoding.DecodeString(s)
				if err != nil {
					log.Fatalf("Failed to decode Base64 string: %s: %v", s, err)
					return
				}
			}
			log.Printf(" Event Kwarsg[0]: %s", string(bs))
		}
	}
}
