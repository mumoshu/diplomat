package main

import (
	"context"
	"github.com/gammazero/nexus/client"
	"github.com/gammazero/nexus/wamp"
	"log"
)

func PrintBody(f func(context.Context, wamp.List, wamp.Dict, wamp.Dict) *client.InvokeResult) func(context.Context, wamp.List, wamp.Dict, wamp.Dict) *client.InvokeResult {
	return func(ctx context.Context, args wamp.List, kwargs wamp.Dict, details wamp.Dict) *client.InvokeResult {
		body := kwargs["body"]
		log.Printf("Request body received: %s", body)
		return f(ctx, args, kwargs, details)
	}
}
