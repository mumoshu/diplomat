package main

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

// This is the body of data that is sent in chunks.
var gettysburg = `Four score and seven years ago our fathers brought forth on this continent, a new nation, conceived in Liberty, and dedicated to the proposition that all men are created equal.
Now we are engaged in a great civil war, testing whether that nation, or any nation so conceived and dedicated, can long endure. We are met on a great battle-field of that war. We have come to dedicate a portion of that field, as a final resting place for those who here gave their lives that that nation might live. It is altogether fitting and proper that we should do this.
But, in a larger sense, we can not dedicate -- we can not consecrate -- we can not hallow -- this ground. The brave men, living and dead, who struggled here, have consecrated it, far above our poor power to add or detract. The world will little note, nor long remember what we say here, but it can never forget what they did here. It is for us the living, rather, to be dedicated here to the unfinished work which they who fought here have thus far so nobly advanced. It is rather for us to be here dedicated to the great task remaining before us -- that from these honored dead we take increased devotion to that cause for which they gave the last full measure of devotion -- that we here highly resolve that these dead shall not have died in vain -- that this nation, under God, shall have a new birth of freedom -- and that government of the people, by the people, for the people, shall not perish from the earth.`
