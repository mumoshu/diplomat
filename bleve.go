package main

import (
	"fmt"
	"github.com/blevesearch/bleve"
)

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

