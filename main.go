package main

import (
	"bufio"
	"log"
	"net"
	"net/http"
	_ "net/http/pprof"
	"os"
	"os/signal"
	"runtime"
)

func main() {
	l, err := net.Listen("tcp", ":8080")
	if err != nil {
		log.Fatalf("Error starting tcp server %s \n", err)
	}
	defer func(l net.Listener) {
		err := l.Close()
		if err != nil {
			log.Printf("error closing listener: %s", err)
		}
	}(l)

	runtime.SetBlockProfileRate(1)
	go func() {
		log.Println(http.ListenAndServe("localhost:6060", nil))
	}()

	outboundMsgs := make(chan Msg)
	newConns := make(chan *Conn)
	conns := make(map[*Conn]bool)
	go func() {
		var id Id = 1
		for {
			rawConn, err := l.Accept()
			if err != nil {
				log.Printf("Error accepting new connetion: %s \n", err)
			}
			inboundMsgs := make(chan Msg)
			newConns <- &Conn{
				Id:           id,
				ReadWrite:    bufio.NewReadWriter(bufio.NewReader(rawConn), bufio.NewWriter(rawConn)),
				ReadFromConn: outboundMsgs,
				WriteToConn:  inboundMsgs,
			}
			id++
		}
	}()

	go func() {
		for c := range newConns {
			go launchReader(c)
			go launchWriter(c)
			conns[c] = true
		}
	}()

	go func() {
		for m := range outboundMsgs {
			for conn := range conns {
				conn.WriteToConn <- m
			}
		}
	}()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, os.Kill)
	<-sig
	log.Println("Shutting Down...")
}

func launchReader(c *Conn) {
	log.Printf("launching new reader id %d", c.Id)
	for {
		m, err := c.ReadWrite.ReadString('\n')
		if err != nil {
			log.Printf("error reading connection id: %d", c.Id)
			continue
		}
		c.ReadFromConn <- Msg{Content: m, SenderId: c.Id}

		//FIXME exiting this goroutine
	}
}

func launchWriter(c *Conn) {
	log.Printf("launching new writer id %d", c.Id)
	for m := range c.WriteToConn {
		if c.Id == m.SenderId {
			continue
		}

		_, err := c.ReadWrite.WriteString(m.Content)
		if err != nil {
			log.Printf("error writing msg to connection %d", c.Id)
		}
		err = c.ReadWrite.Flush()
		if err != nil {
			log.Printf("error flushing connection %d", c.Id)
		}
	}

	//FIXME exiting this goroutine
}
