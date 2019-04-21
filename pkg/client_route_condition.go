package diplomat

import (
	"fmt"
	"github.com/mumoshu/diplomat/pkg/api"
	"net/url"
)

type CondBuilder struct {
	Channel api.ChannelRef
	Path    []string
	ParameterName string
}

func On(ch api.ChannelRef) CondBuilder {
	return CondBuilder{
		Channel: ch,
	}
}

func OnURL(u string) CondBuilder {
	parsed, err := url.Parse(u)
	if err != nil {
		panic(err)
	}
	return CondBuilder{
		Channel: api.ChannelRef{
			Scheme:      api.Scheme(parsed.Scheme),
			ChannelName: fmt.Sprintf("%s%s", parsed.Host, parsed.Path),
		},
	}
}

func (b CondBuilder) Parameter(name string) CondBuilder {
	b.ParameterName = name
	return b
}

func (b CondBuilder) All() RouteCondition {
	c := RouteCondition{
		Channel: b.Channel,
		FormParameterName: b.ParameterName,
	}
	c.Expressions = []Expr{{Path: b.Path, All: true}}
	return c
}

func (b CondBuilder) Where(path ...string) CondBuilder {
	b.Path = path
	return b
}

func (b CondBuilder) EqInt(v int) RouteCondition {
	c := RouteCondition{
		Channel: b.Channel,
		FormParameterName: b.ParameterName,
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
		FormParameterName: b.ParameterName,
	}
	if b.Path != nil {
		c.Expressions = []Expr{{Path: b.Path, String: &s}}
	} else {
		c.Expressions = []Expr{}
	}
	return c
}

