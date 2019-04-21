package diplomat

import (
	"context"
	"fmt"
	"github.com/gammazero/nexus/client"
	"github.com/gammazero/nexus/wamp"
	"github.com/mumoshu/diplomat/pkg/api"
	"log"
	"net/http"
	"time"
)

type Client struct {
	*client.Client
}

//func Serve(srv *Server, cond RouteCondition, func(evt []byte) ([]byte, error) {
//	topic1 := srv.AddConditionalRouteToTopic(cond)
//
//}
//

func (c *Client) startRouting(reg RouteConfig) error {
	fmt.Printf("client: registering %v\n", reg)
	_, err := Call(c.Client, api.ChannelStartRouting.SendChannelURL(), reg)
	if err != nil {
		return fmt.Errorf("registration failed: %v", err)
	}
	return nil
}

func (c *Client) stopRouting(reg RouteConfig) error {
	fmt.Printf("client: stopping routing %v\n", reg)
	_, err := Call(c.Client, api.ChannelStopRouting.SendChannelURL(), reg)
	if err != nil {
		return fmt.Errorf("stopping routing failed: %v", err)
	}
	return nil
}

func (c *Client) StopServing(cond RouteCondition) error {
	proc := cond.ReceiverName()
	if err := c.Unregister(proc); err != nil {
		return fmt.Errorf("failed to unregister %s: %v", proc, err)
	}
	return c.stopRouting(RouteConfig{RouteCondition: cond, Proc: true, Topic: false})
}

func (c *Client) StopSubscription(cond RouteCondition) error {
	topic := cond.ReceiverName()
	if err := c.Unsubscribe(topic); err != nil {
		return fmt.Errorf("failed to unsubscribe %s: %v", topic, err)
	}
	return c.stopRouting(RouteConfig{RouteCondition: cond, Proc: false, Topic: true})
}

func (c *Client) ServeAny(cond RouteCondition, f func(in interface{}) (interface{}, error)) error {
	if err := c.startRouting(RouteConfig{RouteCondition: cond, Proc: true, Topic: false}); err != nil {
		return fmt.Errorf("registration failed: %v", err)
	}
	return c.serve(cond, f)
}

func (c *Client) serve(cond RouteCondition, f func(in interface{}) (interface{}, error)) error {
	cli := c.Client
	handler := c.anyFuncToProcHandler(f)
	ch := cond.Channel
	proc := cond.ReceiverName()
	if err := cli.Register(proc, handler, make(wamp.Dict)); err != nil {
		return fmt.Errorf("Failed to register %q: %s", ch, err)
	}

	log.Printf("Registered procedure %s for channel %s with router", proc, ch)
	return nil
}

func (c *Client) anyFuncToProcHandler(f func(in interface{}) (interface{}, error)) func(context.Context, wamp.List, wamp.Dict, wamp.Dict) *client.InvokeResult {
	return func(ctx context.Context, args wamp.List, kwargs wamp.Dict, details wamp.Dict) *client.InvokeResult {
		req := args[0]
		res, err := f(req)
		if err != nil {
			return &client.InvokeResult{Err: wamp.ErrInvalidArgument, Kwargs: wamp.Dict{"message": fmt.Sprintf("unexpected error: %v", err)}}
		}
		return &client.InvokeResult{Args: wamp.List{res}}
	}
}

func (c *Client) anyFuncToSubscriptionHandler(f func(in interface{})) client.EventHandler {
	return func(args wamp.List, kwargs wamp.Dict, details wamp.Dict) {
		req := kwargs["body"]
		f(req)
	}
}

func (c *Client) ServeWithProgress(cond RouteCondition, f func(evt []byte) ([]byte, error)) (<-chan struct{}, error) {
	reg := RouteConfig{RouteCondition: cond, Proc: true, Topic: false,}
	if err := c.startRouting(reg); err != nil {
		log.Fatalf("registration failed 1: %v", err)
	}

	return c.listenAndServeWithProgress(cond, f)
}

func (c *Client) SubscribeAny(cond RouteCondition, f func(evt interface{})) error {
	reg := RouteConfig{RouteCondition: cond, Proc: false, Topic: true,}
	if err := c.startRouting(reg); err != nil {
		log.Fatalf("subscription registration failed : %v", err)
	}

	return c.subscribeAny(cond, f)
}

func (c *Client) subscribeAny(cond RouteCondition, f func(evt interface{})) error {
	err := c.Client.Subscribe(cond.ReceiverName(), c.anyFuncToSubscriptionHandler(f), nil)
	if err != nil {
		return fmt.Errorf("subscription failed:", err)
	}
	log.Printf("%s subscribed to %s", c.ID(), cond.ReceiverName())
	return nil
}

type HttpHandler interface {
	ServeHTTP(w http.ResponseWriter, r *http.Request)
}

func (c *Client) SubscribeHTTP(url string, cond RouteCondition, handler HttpHandler) error {
	conf := RouteConfig{RouteCondition: cond, Proc: false, Topic: true,}
	if err := c.startRouting(conf); err != nil {
		log.Fatalf("slack subscription registration failed : %v", err)
	}

	return c.subscribeHttp(url, cond, handler)
}

func (c *Client) ServeHTTP(url string, cond RouteCondition, handler HttpHandler) error {
	conf := RouteConfig{RouteCondition: cond, Proc: true, Topic: false}
	if err := c.startRouting(conf); err != nil {
		return fmt.Errorf("slack serve registration failed: %v", err)
	}
	return c.serveHttp(url, cond, handler)
}

func (c *Client) serveHttp(uu string, cond RouteCondition, f http.Handler) error {
	cli := c.Client
	handler, err := httpInvocationHandlerAdapter(uu, f)
	if err != nil {
		return err
	}
	ch := cond.Channel
	proc := cond.ReceiverName()
	if err := cli.Register(proc, handler, make(wamp.Dict)); err != nil {
		return fmt.Errorf("Failed to register %q: %s", ch, err)
	}

	log.Printf("Registered procedure %s for channel %s with router", proc, ch)
	return nil
}

func (c *Client) subscribeHttp(uu string, cond RouteCondition, f HttpHandler) error {
	var handler client.EventHandler

	handler, err := httpHandlerAdapter(uu, f, nil)
	if err != nil {
		return err
	}
	err = c.Client.Subscribe(cond.ReceiverName(), handler, nil)
	if err != nil {
		return fmt.Errorf("subscription failed:", err)
	}
	log.Printf("%s subscribed to %s", c.ID(), cond.ReceiverName())
	return nil
}

func (c *Client) listenAndServeWithProgress(cond RouteCondition, f func(evt []byte) ([]byte, error)) (<-chan struct{}, error) {
	//proc1 := srv.AddConditionalRouteToProcedure(cond)

	procName := cond.ReceiverName()

	cli := c.Client

	//call(locallCalee, "AddConditionalRouteToProcedure", )

	localCalleeHandler := PrintBody(func(ctx context.Context, args wamp.List, kwargs wamp.Dict, details wamp.Dict) *client.InvokeResult {
		bs, err := getBodyBytes(kwargs)
		if err != nil {
			log.Fatalf("print body failed: %v", err)
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
