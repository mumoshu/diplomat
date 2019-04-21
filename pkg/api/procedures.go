package api

import "fmt"

var ChannelStartRouting ChannelRef
var ChannelStopRouting ChannelRef
var ChannelEcho ChannelRef

type Scheme string

var SchemeDiplomat = Scheme("diplomat")

type ChannelRef struct {
	Scheme      Scheme
	ChannelName string
}

func (c ChannelRef) SendChannelURL() string {
	return string(c.String())
}

func (id ChannelRef) String() string {
	return fmt.Sprintf("%s://%s", id.Scheme, id.ChannelName)
}

func init() {
	ChannelStartRouting = ChannelRef{
		Scheme:      SchemeDiplomat,
		ChannelName: "register",
	}
	ChannelStopRouting = ChannelRef{
		Scheme:      SchemeDiplomat,
		ChannelName: "stopRouting",
	}
	ChannelEcho = ChannelRef{
		Scheme:      SchemeDiplomat,
		ChannelName: "echo",
	}
}
