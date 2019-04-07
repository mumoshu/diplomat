package main

import (
	"context"
	"fmt"
	"github.com/gammazero/nexus/client"
	"github.com/gammazero/nexus/wamp"
	"log"
	"time"
)

type CondBuilder struct {
	Path []string
}

func When(path ...string) CondBuilder {
	return CondBuilder{
		Path: path,
	}
}

func (b CondBuilder) EqInt(v int) RouteCondition {
	return RouteCondition{
		Expr{Path: b.Path, Int: &v},
	}
}

func (b CondBuilder) EqString(s string) RouteCondition {
	return RouteCondition{
		Expr{Path: b.Path, String: &s},
	}
}

//func Serve(srv *Server, cond RouteCondition, func(evt []byte) ([]byte, error) {
//	topic1 := srv.AddConditionalRouteToTopic(cond)
//
//}
//

func Serve(srv ServerRef, cond RouteCondition, f func(evt []byte) ([]byte, error)) (<-chan struct{}, error) {
	//proc1 := srv.AddConditionalRouteToProcedure(cond)

	procName := cond.Proc()
	clientName := procName
	cli, err := srv.Connect(clientName)
	if err != nil {
		log.Fatal(err)
	}

	//call(locallCalee, "AddConditionalRouteToProcedure", )

	localCalleeHandler := PrintBody(func(ctx context.Context, args wamp.List, kwargs wamp.Dict, details wamp.Dict) *client.InvokeResult {
		bs, ok := kwargs["body"].([]byte)
		if !ok {
			return &client.InvokeResult{Err: wamp.ErrInvalidArgument}
		}
		data, err := f(bs)
		if err != nil {
			return &client.InvokeResult{Err: wamp.ErrInvalidArgument}
		}
		return progressiveSend(ctx, cli, data, args)
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

	if err = cli.Register(procName, localCalleeHandler, make(wamp.Dict)); err != nil {
		return nil, fmt.Errorf("Failed to register %q: %s", procName, err)
	}

	log.Printf("Registered procedure %q with router", procName)
	return cli.Done(), nil
}