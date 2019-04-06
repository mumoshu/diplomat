package main

import (
	"github.com/gammazero/nexus/wamp"
	"log"
)

func createEvtHandler(topic string) func(args wamp.List, kwargs wamp.Dict, details wamp.Dict) {
	return func(args wamp.List, kwargs wamp.Dict, details wamp.Dict) {
		log.Println("Received", topic, "event")
		if len(args) != 0 {
			log.Println("  Event Arg[0]:", args[0])
		}
		if len(kwargs) != 0 {
			log.Printf(" Event Kwarsg: %v", kwargs)
		}
	}
}
