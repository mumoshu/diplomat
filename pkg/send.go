package diplomat

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"github.com/gammazero/nexus/client"
	"github.com/gammazero/nexus/wamp"
)

// progressiveSend sends the body of data in chunks of the requested size.  The final
// result message contains the sha256 hash of the data to allow the caller to
// verify that all the data was correctly received.
func progressiveSend(ctx context.Context, callee *client.Client, data []byte, args wamp.List) *client.InvokeResult {
	// Compute the base64-encoded sha256 hash of the data.
	h := sha256.New()
	h.Write(data)
	hash64 := base64.StdEncoding.EncodeToString(h.Sum(nil))

	// Put data in buffer to read chunks from.
	b := bytes.NewBuffer(data)

	// Get chunksize requested by caller, use default if not set.
	var chunkSize int
	if len(args) != 0 {
		i, _ := wamp.AsInt64(args[0])
		chunkSize = int(i)
	}
	if chunkSize == 0 {
		chunkSize = 64
	}

	// Read and send chunks of data until the buffer is empty.
	for chunk := b.Next(chunkSize); len(chunk) != 0; chunk = b.Next(chunkSize) {
		// Send a chunk of data.
		err := callee.SendProgress(ctx, wamp.List{string(chunk)}, nil)
		if err != nil {
			// If send failed, return an error saying the call canceled.
			return &client.InvokeResult{Err: wamp.ErrCanceled}
		}
	}

	// Send sha256 hash as final result.
	return &client.InvokeResult{Args: wamp.List{hash64}}
}

func send(ctx context.Context, callee *client.Client, data interface{}) *client.InvokeResult {
	return &client.InvokeResult{Args: wamp.List{data}}
}
