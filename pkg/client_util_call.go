package diplomat

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"github.com/gammazero/nexus/client"
	"github.com/gammazero/nexus/wamp"
	"log"
	"strings"
)

func ProgressiveCall(caller *client.Client, procedureName string, evt Event, chunkSize int) (*Output, error) {
	kwargs := eventToKwargs(evt)
	res, err := progressiveCall(caller, procedureName, kwargs, chunkSize)
	if err != nil {
		return nil, fmt.Errorf("progressive call failed: %v", err)
	}
	return kwargsToOutput(res.ArgumentsKw)
}

func progressiveCall(caller *client.Client, procedureName string, kwargs wamp.Dict, chunkSize int) (*wamp.Result, error) {
	// The progress handler accumulates the chunks of data as they arrive.  It
	// also progressively calculates a sha256 hash of the data as it arrives.
	var chunks []string
	h := sha256.New()
	progHandler := func(result *wamp.Result) {
		// Received another chunk of data, computing hash as chunks received.
		chunk := result.Arguments[0].(string)
		log.Println("Received", len(chunk), "bytes (as progressive result)")
		log.Println(string(chunk))
		chunks = append(chunks, chunk)
		h.Write([]byte(chunk))
	}

	ctx := context.Background()

	// Call the example procedure, specifying the size of chunks to send as
	// progressive results.
	result, err := caller.CallProgress(
		ctx, procedureName, nil, wamp.List{chunkSize}, kwargs, "", progHandler)
	if err != nil {
		return nil, fmt.Errorf("Failed to call procedure:", err)
	}

	var res []byte

	// As a final result, the callee returns the base64 encoded sha256 hash of
	// the data.  This is decoded and compared to the value that the caller
	// calculated.  If they match, then the caller recieved the data correctly.
	if len(result.Arguments) > 0 {
		hashB64 := result.Arguments[0].(string)
		log.Printf("Received hashB64: %s", hashB64)
		calleeHash, err := base64.StdEncoding.DecodeString(hashB64)
		if err != nil {
			return nil, fmt.Errorf("decode error:", err)
		}
		// Check if received hash matches the hash computed over the received data.
		if !bytes.Equal(calleeHash, h.Sum(nil)) {
			return nil, fmt.Errorf("Hash of received data does not match")
		}
		res = []byte(strings.Join(chunks, ""))
	} else {
		resBody, err := getBodyBytes(result.ArgumentsKw)
		if err != nil {
			return nil, fmt.Errorf("failed to get body: %v", err)
		}
		res = resBody
	}

	if result.ArgumentsKw == nil {
		result.ArgumentsKw = wamp.Dict{}
	}
	result.ArgumentsKw["body"] = res

	log.Println("Correctly received data from callee:")
	log.Println("----------------------------")
	log.Println(string(res))

	return result, nil
}

func Call(caller *client.Client, procedure string, evt interface{}) (interface{}, error) {
	return call(caller, procedure, evt)
}

func call(caller *client.Client, procedureName string, evt interface{}) (interface{}, error) {
	ctx := context.Background()

	// Call the example procedure, specifying the size of chunks to send as
	// progressive results.
	result, err := caller.Call(
		ctx, procedureName, nil, wamp.List{evt}, wamp.Dict{}, "")
	if err != nil {
		return nil, fmt.Errorf("Failed to call procedure: %v", err)
	}

	return result.Arguments[0], nil
}
