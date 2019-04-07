package main

import (
	"encoding/base64"
	"fmt"
	"github.com/gammazero/nexus/wamp"
	"log"
)

func getBodyBytes(kwargs wamp.Dict) ([]byte, error) {
	bs, ok := kwargs["body"].([]byte)
	if !ok {
		log.Printf("Decoding base64: %v", kwargs["body"])
		s, isStr := kwargs["body"].(string)
		if !isStr {
			return nil, fmt.Errorf("Unexpected body: %T: %v", kwargs["body"], kwargs["body"])
		}
		var err error
		bs, err = base64.StdEncoding.DecodeString(s)
		if err != nil {
			return nil, fmt.Errorf("Failed to decode Base64 string: %s: %v", s, err)
		}
	}
	return bs, nil
}

func createEvtHandler(subId, topic string) func(args wamp.List, kwargs wamp.Dict, details wamp.Dict) {
	return func(args wamp.List, kwargs wamp.Dict, details wamp.Dict) {
		log.Println("Subscriber", subId, "received ", topic, "event")
		if len(args) != 0 {
			log.Println("  Event Arg[0]:", args[0])
		}
		if len(kwargs) != 0 {
			bs, err := getBodyBytes(kwargs)
			if err != nil {
				log.Fatalf("Unexpected error: %v", err)
			}
			log.Printf(" Event Kwarsg[0]: %s", string(bs))
		}
	}
}
