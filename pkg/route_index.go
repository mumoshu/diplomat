package diplomat

import (
	"encoding/json"
	"fmt"
	"github.com/valyala/fastjson"
	"log"
	"net/url"
	"os"
	"strings"
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
	SendChannelToJsonHandlerIndex          map[string]*ContentBasedRouteIndex
	SendChannelToFormParameterHandlerIndex map[string]map[string]*ContentBasedRouteIndex
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
	if idx.SendChannelToJsonHandlerIndex == nil {
		idx.SendChannelToJsonHandlerIndex = map[string]*ContentBasedRouteIndex{}
	}
	if idx.SendChannelToFormParameterHandlerIndex == nil {
		idx.SendChannelToFormParameterHandlerIndex = map[string]map[string]*ContentBasedRouteIndex{}
	}
	if r.FormParameterName == "" {
		_, ok := idx.SendChannelToJsonHandlerIndex[ch]
		if !ok {
			idx.SendChannelToJsonHandlerIndex[ch] = &ContentBasedRouteIndex{
				Root: &Node{
					Matchers: []*Matcher{},
					Children: map[string]*Node{},
				},
			}
		}
		idx.SendChannelToJsonHandlerIndex[ch].Index(r)
	} else {
		_, ok := idx.SendChannelToFormParameterHandlerIndex[ch]
		if !ok {
			idx.SendChannelToFormParameterHandlerIndex[ch] = map[string]*ContentBasedRouteIndex{
			}
		}
		_, ok2 := idx.SendChannelToFormParameterHandlerIndex[ch][r.FormParameterName]
		if !ok2 {
			idx2 := &ContentBasedRouteIndex{
				Root: &Node{
					Matchers: []*Matcher{},
					Children: map[string]*Node{},
				},
			}
			idx.SendChannelToFormParameterHandlerIndex[ch][r.FormParameterName] = idx2
		}
		idx.SendChannelToFormParameterHandlerIndex[ch][r.FormParameterName].Index(r)
	}
}

func (idx *RouteIndex) SearchRouteMatchesChannelAndJSON(ch string, data []byte) (map[RouteConditionID]int, error) {
	cidx, ok := idx.SendChannelToJsonHandlerIndex[ch]
	if !ok {
		_, ok2 := idx.SendChannelToFormParameterHandlerIndex[ch]
		if !ok2 {
			return nil, fmt.Errorf("unknown channel: no route registered for this channel. please run a server that responds to this channel")
		}
		// parse request body
		str := string(data)
		kvslice := strings.Split(str, "&")
		kvs := map[string]string{}
		for _, kv := range kvslice {
			split := strings.Split(kv, "=")
			k := split[0]
			v := split[1]
			kvs[k] = v
		}
		for param, idx2 := range idx.SendChannelToFormParameterHandlerIndex[ch] {
			v, ok3 := kvs[param]
			if !ok3 {
				continue
			}
			vAsJson, err := url.QueryUnescape(v)
			score, err := idx2.SearchRouteMatchesJSON([]byte(vAsJson))
			if len(score) > 0 {
				return score, err
			}
			if err != nil {
				fmt.Fprintf(os.Stderr, "ignoring err: %v", err)
			}
		}
		return nil, fmt.Errorf("no match for data: %s", string(data))
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
