package main

import (
	"fmt"
	"github.com/minio/highwayhash"
	"sort"
	"strconv"
	"strings"
)

type Expr struct {
	Path   []string
	String *string
	Int *int
}

type RouteCondition []Expr

func (c RouteCondition) Topic() string {
	return string("topic-" + c.ID())
}

func (c RouteCondition) Proc() string {
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
	for _, c := range m {
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
	return RouteConditionID(strings.Join(idparts, "&"))
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

