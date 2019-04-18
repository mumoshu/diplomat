package diplomat

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

type ContentBasedRouteIndex struct {
	parserPool fastjson.ParserPool

	Root *Node
}

type RouteIndex struct {
	ChannelToIndex map[string]*ContentBasedRouteIndex
}

func newNode() *Node {
	return &Node{
		Matchers: []*Matcher{},
		Children: map[string]*Node{},
	}
}

func (idx *ContentBasedRouteIndex) getNode(path []string) *Node {
	node := idx.Root
	for _, k := range path {
		if node.Children[k] == nil {
			node.Children[k] = newNode()
		}
		node = node.Children[k]
	}
	return node
}

func (idx *ContentBasedRouteIndex) Index(r *Route) {
	for _, cond := range r.RouteCondition.Expressions {
		id := r.ID()
		m := &Matcher{
			String:           cond.String,
			Int:              cond.Int,
			RouteConditionID: id,
		}
		node := idx.getNode(cond.Path)
		node.Matchers = append(node.Matchers, m)
	}
	dump, err := json.Marshal(*idx.Root)
	if err != nil {
		log.Fatalf("failed to dump index: %v", err)
	}
	log.Printf("Index updated: %s", string(dump))
}

func (idx *ContentBasedRouteIndex) SearchRouteMatchesJSON(data []byte) (map[RouteConditionID]int, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("search failed: empty body")
	}
	parser := idx.parserPool.Get()
	defer idx.parserPool.Put(parser)
	jsonValue, err := parser.ParseBytes(data)
	if err != nil {
		return nil, err
	}
	return idx.Root.search(&SearchContext{map[RouteConditionID]int{}}, jsonValue)
}

func (idx *RouteIndex) Index(r *Route) {
	ch := r.Channel.SendChannelURL()
	if idx.ChannelToIndex == nil {
		idx.ChannelToIndex = map[string]*ContentBasedRouteIndex{}
	}
	_, ok := idx.ChannelToIndex[ch]
	if !ok {
		idx.ChannelToIndex[ch] = &ContentBasedRouteIndex{
			Root: &Node{
				Matchers: []*Matcher{},
				Children: map[string]*Node{},
			},
		}
	}
	idx.ChannelToIndex[ch].Index(r)
}


func (idx *RouteIndex) SearchRouteMatchesChannelAndJSON(ch string, data []byte) (map[RouteConditionID]int, error) {
	cidx, ok := idx.ChannelToIndex[ch]
	if !ok {
		return nil, fmt.Errorf("unknown channel: no route registered for this channel. please run a server that responds to this channel")
	}
	return cidx.SearchRouteMatchesJSON(data)
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
