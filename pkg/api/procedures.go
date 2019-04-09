package api

import "fmt"

var DiplomatRegisterChan ChannelRef
var DiplomatEchoChan ChannelRef

type Scheme string

var SchemeDiplomat = Scheme("diplomat")

type ChannelRef struct {
	Scheme      Scheme
	ChannelName string
}

func (id ChannelRef) String() string {
	return fmt.Sprintf("%s:///%s", id.Scheme, id.ChannelName)
}

func init() {
	DiplomatRegisterChan = ChannelRef{
		Scheme:      SchemeDiplomat,
		ChannelName: "register",
	}
	DiplomatRegisterChan = ChannelRef{
		Scheme:      SchemeDiplomat,
		ChannelName: "echo",
	}
}
