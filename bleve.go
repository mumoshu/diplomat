package main

import (
	"fmt"
	"github.com/blevesearch/bleve"
	"sort"
)

type Query struct {
	Data map[string]string
}

func (q Query) String() string {
	keys := make([]string, len(q.Data))
	i := 0
	for k := range q.Data {
		keys[i] = k
		i ++
	}
	sort.Strings(keys)
	s := ""
	for i := range keys {
		if s != "" {
			s += " "
		}
		k := keys[i]
		s += "+" + k + ":" + q.Data[k]
	}
	return s
}

func index1(data map[string]string) error {
	q := Query{data}
	// open a new index
	mapping := bleve.NewIndexMapping()
	index, err := bleve.New("example.bleve", mapping)
	if err != nil {
		return err
	}

	// index some data
	queryStr := q.String()
	err = index.Index(queryStr, q)

	// search for some text
	query := bleve.NewMatchQuery("text")
	search := bleve.NewSearchRequest(query)
	searchResults, err := index.Search(search)
	fmt.Printf("%s", searchResults)
	return nil
}

func mapsearch() {
	message := struct {
		Id   string
		From string
		Body string
	}{
		Id:   "example",
		From: "marty.schoch@gmail.com",
		Body: "bleve indexing is easy",
	}

	mapping := bleve.NewIndexMapping()
	index, err := bleve.New("example.bleve", mapping)
	if err != nil {
		panic(err)
	}
	index.Index(message.Id, message)

	// query
	index, _ = bleve.Open("example.bleve")
	query := bleve.NewQueryStringQuery("bleve")
	searchRequest := bleve.NewSearchRequest(query)
	searchResult, _ := index.Search(searchRequest)
	println(index, searchResult)
}

