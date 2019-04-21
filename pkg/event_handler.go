package diplomat

import (
	"github.com/gammazero/nexus/wamp"
	"log"
)

func PrintingHandler(subId, topic string) func(args wamp.List, kwargs wamp.Dict, details wamp.Dict) {
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
