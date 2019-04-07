package main

import (
	"encoding/json"
	"fmt"
	"github.com/valyala/fastjson"
	"log"
)

type Matcher struct {
	String           *string
	Int              *int
	RouteConditionID RouteConditionID
}

type Node struct {
	Matchers []*Matcher

	Children map[string]*Node
}

type RouteIndex struct {
	parserPool fastjson.ParserPool

	root *Node
}

func newNode() *Node {
	return &Node{
		Matchers: []*Matcher{},
		Children: map[string]*Node{},
	}
}

func (idx *RouteIndex) getNode(path []string) *Node {
	node := idx.root
	for _, k := range path {
		if node.Children[k] == nil {
			node.Children[k] = newNode()
		}
		node = node.Children[k]
	}
	return node
}

func (idx *RouteIndex) Index(r *Route) {
	for _, cond := range r.RouteCondition {
		id := r.ID()
		m := &Matcher{
			String:           cond.String,
			Int:              cond.Int,
			RouteConditionID: id,
		}
		node := idx.getNode(cond.Path)
		node.Matchers = append(node.Matchers, m)
	}
	dump, err := json.Marshal(*idx.root)
	if err != nil {
		log.Fatalf("failed to dump index: %v", err)
	}
	log.Printf("Index updated: %s", string(dump))
}

func (idx *RouteIndex) Search(data []byte) (map[RouteConditionID]int, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("search failed: empty body")
	}
	parser := idx.parserPool.Get()
	defer idx.parserPool.Put(parser)
	v, err := parser.ParseBytes(data)
	if err != nil {
		return nil, err
	}
	return idx.root.search(&SearchContext{map[RouteConditionID]int{}}, v)
}

type SearchContext struct {
	Scores map[RouteConditionID]int
}

func (node *Node) search(ctx *SearchContext, v *fastjson.Value) (map[RouteConditionID]int, error) {
	var strv *string
	var intv *int
	if !v.Exists() {
		return ctx.Scores, nil
	}
	for _, m := range node.Matchers {
		if m.String != nil {
			if strv == nil {
				strbytes := v.GetStringBytes()
				if strbytes != nil {
					s := string(strbytes)
					strv = &s
				}
			}
			if strv != nil && *strv == *m.String {
				ctx.Scores[m.RouteConditionID] += 1
			}
		}

		if m.Int != nil {
			if intv == nil {
				intr, err := v.Int()
				if err == nil {
					intv = &intr
				}
			}
			if intv != nil && *intv == *m.Int {
				ctx.Scores[m.RouteConditionID] += 1
			}
		}
	}
	for key, child := range node.Children {
		cv := v.Get(key)
		_, err := child.search(ctx, cv)
		if err != nil {
			return nil, err
		}
	}
	return ctx.Scores, nil
}
