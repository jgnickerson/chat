package main

import (
	"net"
)

type Id int

type Msg struct {
	SenderId Id
	Content string
}

type Conn struct {
	Id Id
	RawConn   net.Conn
	// Msgs coming from this Connection, to go to other connections
	ReadFromConn chan Msg
	// Inbound from this connection
	WriteToConn chan Msg
}
