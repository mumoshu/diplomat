package diplomat

import (
	"encoding/base64"
	"fmt"
	"github.com/gammazero/nexus/wamp"
	"log"
	"net/http"
)

func getBodyBytes(kwargs wamp.Dict) ([]byte, error) {
	bs, ok := kwargs["body"].([]byte)
	if !ok {
		log.Printf("Decoding base64: %v", kwargs["body"])
		s, isStr := kwargs["body"].(string)
		if !isStr {
			return nil, fmt.Errorf("Unexpected body: %T: %v", kwargs["body"], kwargs["body"])
		}
		var err error
		bs, err = base64.StdEncoding.DecodeString(s)
		if err != nil {
			return nil, fmt.Errorf("Failed to decode Base64 string: %s: %v", s, err)
		}
	}
	return bs, nil
}

func eventToKwargs(evt Event) wamp.Dict {
	return wamp.Dict{
		"body":   evt.Body,
		"header": evt.Header,
	}
}

func kwargsToOutput(kwargs wamp.Dict) (*Output, error) {
	bytes, err := getBodyBytes(kwargs)
	if err != nil {
		return nil, fmt.Errorf("kwargsToOutput failed: %v", err)
	}
	return &Output{
		Body:       bytes,
		Header:     getHeader(kwargs),
		StatusCode: getStatusCode(kwargs),
	}, nil
}

func getStatusCode(kwargs wamp.Dict) int {
	if kwargs["statusCode"] != nil {
		switch typed := kwargs["statusCode"].(type) {
		case uint64:
			return int(typed)
		default:
			panic(fmt.Errorf("unexpected type of status code in %v: type=%T code=%v", kwargs, typed, typed))
		}
	}
	return 0
}

func getHeader(kwargs wamp.Dict) map[string][]string {
	if kwargs["header"] != nil {
		switch typed := kwargs["header"].(type) {
		case map[string]interface{}:
			header := map[string][]string{}
			for k, v := range typed {
				sli := v.([]interface{})
				vs := []string{}
				for _, v := range sli {
					vs = append(vs, v.(string))
				}
				header[k] = vs
			}
			return header
		default:
			panic(fmt.Sprintf("unexpected type of header %T: %v", typed, typed))
		}
	}
	return map[string][]string{}
}

func getHttpHeader(kwargs wamp.Dict) http.Header {
	if kwargs["header"] != nil {
		switch typed := kwargs["header"].(type) {
		case map[string]interface{}:
			header := http.Header{}
			for k, v := range typed {
				sli := v.([]interface{})
				vs := []string{}
				for _, v := range sli {
					vs = append(vs, v.(string))
				}
				header[k] = vs
			}
			return header
		default:
			panic(fmt.Sprintf("unexpected type of header %T: %v", typed, typed))
		}
	}
	return http.Header{}
}
