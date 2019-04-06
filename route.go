package main

import (
	"github.com/minio/highwayhash"
	"sort"
	"strings"
)

type Expr struct {
	Path   []string
	String *string
	Int *int
}

type RouteCondition []Expr

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
	for _, c := range m {
		for _, key := range c.Path {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	return RouteConditionID(strings.Join(keys, ""))
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

