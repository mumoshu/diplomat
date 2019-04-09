package diplomat

import (
	"context"
	"fmt"
	"github.com/gammazero/nexus/client"
	"github.com/gammazero/nexus/wamp"
	"github.com/mumoshu/diplomat/pkg/api"
	"log"
	"time"
)

type CondBuilder struct {
	Channel api.ChannelRef
	Path []string
}

func On(ch api.ChannelRef) CondBuilder {
	return CondBuilder{
		Channel: ch,
	}
}

func (b CondBuilder) All() RouteCondition {
	return RouteCondition{
		Channel: b.Channel,
	}
}

func (b CondBuilder) Where(path ...string) CondBuilder {
	b.Path = path
	return b
}

func (b CondBuilder) EqInt(v int) RouteCondition {
	c := RouteCondition{
		Channel: b.Channel,
	}
	if b.Path != nil {
		c.Expressions = []Expr{{Path: b.Path, Int: &v}}
	} else {
		c.Expressions = []Expr{}
	}
	return c
}

func (b CondBuilder) EqString(s string) RouteCondition {
	c := RouteCondition{
		Channel: b.Channel,
	}
	if b.Path != nil {
		c.Expressions = []Expr{{Path: b.Path, String: &s}}
	} else {
		c.Expressions = []Expr{}
	}
	return c
}

type Client struct {
	*client.Client
}

//func Serve(srv *Server, cond RouteCondition, func(evt []byte) ([]byte, error) {
//	topic1 := srv.AddConditionalRouteToTopic(cond)
//
//}
//

func (c *Client) ApplyRegistration(reg Registration) error {
	_, err := Call(c.Client, api.DiplomatRegisterChan, reg)
	if err != nil {
		return fmt.Errorf("registration failed: %v", err)
	}
	return nil
}

func (c *Client) Serve(ch RouteCondition, f func(in interface{}) (interface{}, error)) error {
	cli := c.Client
	handler := c.FuncHandler(f)
	if err := cli.Register(ch.ProcedureName(), handler, make(wamp.Dict)); err != nil {
		return fmt.Errorf("Failed to register %q: %s", ch, err)
	}

	log.Printf("Registered procedure %q with router", ch)
	return nil
}

func (c *Client) FuncHandler(f func(in interface{}) (interface{}, error)) func(context.Context, wamp.List, wamp.Dict, wamp.Dict) *client.InvokeResult {
	return func(ctx context.Context, args wamp.List, kwargs wamp.Dict, details wamp.Dict) *client.InvokeResult {
		req := args[0]
		res, err := f(req)
		if err != nil {
			return &client.InvokeResult{Err: wamp.ErrInvalidArgument, Kwargs: wamp.Dict{"message": fmt.Sprintf("unexpected error: %v", err)}}
		}
		return &client.InvokeResult{Args: wamp.List{res}}
	}
}

func (c *Client) ListenAndServeWithProgress(cond RouteCondition, f func(evt []byte) ([]byte, error)) (<-chan struct{}, error) {
	//proc1 := srv.AddConditionalRouteToProcedure(cond)

	procName := cond.ProcedureName()

	cli := c.Client

	//call(locallCalee, "AddConditionalRouteToProcedure", )

	localCalleeHandler := PrintBody(func(ctx context.Context, args wamp.List, kwargs wamp.Dict, details wamp.Dict) *client.InvokeResult {
		bs, err := getBodyBytes(kwargs)
		if err != nil {
			log.Fatalf("Unexpected error: %v", err)
		}
		if err != nil {
			return &client.InvokeResult{Err: wamp.ErrInvalidArgument, Kwargs: wamp.Dict{"message": fmt.Sprintf("Unexpected type: %T: %v", kwargs["body"], kwargs["body"])}}
		}
		data, err := f(bs)
		if err != nil {
			return &client.InvokeResult{Err: wamp.ErrInvalidArgument, Kwargs: wamp.Dict{"message": err.Error()}}
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

	if err := cli.Register(procName, localCalleeHandler, make(wamp.Dict)); err != nil {
		return nil, fmt.Errorf("Failed to register %q: %s", procName, err)
	}

	log.Printf("Registered procedure %q with router", procName)
	return cli.Done(), nil
}