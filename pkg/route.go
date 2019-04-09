package diplomat

import (
	"encoding/hex"
	"fmt"
	"github.com/minio/highwayhash"
	"github.com/mumoshu/diplomat/pkg/api"
	"sort"
	"strconv"
	"strings"
)

var key []byte

func init() {
	var err error
	key, err = hex.DecodeString("000102030405060708090A0B0C0D0E0FF0E0D0C0B0A090807060504030201000") // use your own key here
	if err != nil {
		fmt.Printf("Cannot decode hex key: %v", err) // add error handling
		return
	}
}

type Expr struct {
	Path   []string
	String *string
	Int *int
}

type RouteCondition struct {
	Channel api.ChannelRef
	Expressions []Expr
}

func (c RouteCondition) TopicName() string {
	return string("topic-" + c.ID())
}

func (c RouteCondition) ProcedureName() string {
	return string("proc-" + c.ID())
}

type Route struct {
	RouteCondition
	Topics     []string
	Procedures []string
}

type RouteConditionID string

func (id RouteConditionID) ID() RouteConditionID {
	return id
}

func (id RouteConditionID) HashValue() uint64 {
	hash, err := highwayhash.New64(key)
	if err != nil {
		panic(err)
	}
	_, err = hash.Write([]byte(id))
	if err != nil {
		panic(err)
	}
	return hash.Sum64()
}

type RouteConditionRef interface {
	ID() RouteConditionID
	HashValue() uint64
}

func (m RouteCondition) ID() RouteConditionID {
	keys := []string{}
	d := map[string]string{}
	for _, c := range m.Expressions {
		var v string
		if c.String != nil {
			v = *c.String
		}
		if c.Int != nil {
			v = strconv.Itoa(*c.Int)
		}
		k := strings.Join(c.Path, ".")
		d[k] = v
		keys = append(keys, k)
	}
	sort.Strings(keys)
	idparts := []string{}
	for _, k := range keys {
		idparts = append(idparts, fmt.Sprintf("%s=%s", k, d[k]))
	}
	query := strings.Join(idparts, "&")
	id := strings.Join([]string{m.Channel.String(), query}, "?")
	return RouteConditionID(id)
}

func (m RouteCondition) HashValue() uint64 {
	return m.ID().HashValue()
}

type RoutesPartition struct {
	Routes map[RouteConditionID]*Route
}

func (ms *RoutesPartition) Get(ref RouteConditionRef) *Route {
	return ms.Routes[ref.ID()]
}

func (ms *RoutesPartition) Put(m *Route) {
	ms.Routes[m.ID()] = m
}

type RouteTable struct {
	RoutePartitions map[uint64]*RoutesPartition
}

type Router struct {
	*RouteTable
}

