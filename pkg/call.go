package diplomat

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/gammazero/nexus/client"
	"github.com/gammazero/nexus/wamp"
	"log"
	"strings"
)

func ProgressiveCall(caller *client.Client, procedureName string, evt interface{}, chunkSize int) ([]byte, error) {
	var bs []byte
	switch typed := evt.(type) {
	case []byte:
		bs = typed
	case string:
		bs = []byte(typed)
	case int, bool, byte:
		return nil, fmt.Errorf("unsupported type of evt: %T: %v", typed, typed)
	default:
		bytes, err := json.Marshal(evt)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal evt: %v", err)
		}
		bs = bytes
	}
	return progressiveCall(caller, procedureName, bs, chunkSize)
}

func progressiveCall(caller *client.Client, procedureName string, evt []byte, chunkSize int) ([]byte, error) {
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
		ctx, procedureName, nil, wamp.List{chunkSize}, wamp.Dict{"body": evt}, "", progHandler)
	if err != nil {
		return nil, fmt.Errorf("Failed to call procedure:", err)
	}

	// As a final result, the callee returns the base64 encoded sha256 hash of
	// the data.  This is decoded and compared to the value that the caller
	// calculated.  If they match, then the caller recieved the data correctly.
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
	log.Println("Correctly received all data:")
	log.Println("----------------------------")
	res := strings.Join(chunks, "")
	log.Println(res)

	return []byte(res), nil
}
