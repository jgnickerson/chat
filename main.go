package main

import (
	"bufio"
	"context"
	"io"
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
				RawConn:      rawConn,
				ReadFromConn: outboundMsgs,
				WriteToConn:  inboundMsgs,
			}
			id++
		}
	}()

	go func() {
		for c := range newConns {
			ctx, cancel := context.WithCancel(context.Background())
			go launchWriter(ctx, c.Id, bufio.NewWriter(c.RawConn), c.WriteToConn)
			c := c
			conns[c] = true
			go func() {
				if err := launchReader(ctx, c.Id, bufio.NewReader(c.RawConn), c.ReadFromConn); err != nil && err != io.EOF {
					log.Printf("error reading from connection %d: %s", c.Id, err)
				}
				cancel()
				if err := c.RawConn.Close(); err != nil {
					log.Printf("error closing connection %d: %s", c.Id, err)
				}
				delete(conns, c)
			}()
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

func launchReader(ctx context.Context, id Id, r *bufio.Reader, msgs chan Msg) error {
	log.Printf("launching new reader id %d", id)
	for {
		if err := ctx.Err(); err == context.Canceled {
			log.Printf("shutting down reader %d", id)
			return nil
		}

		m, err := r.ReadString('\n')
		if err == io.EOF {
			log.Printf("connection %d closed by client", id)
			return err
		}

		//FIXME how to properly handle non-EOF error?
		// Perhaps some sort of exponential backoff and retry?
		if err != nil {
			log.Printf("error reading connection id: %d", id)
			return err
		}
		msgs <- Msg{Content: m, SenderId: id}
	}
}

func launchWriter(ctx context.Context, id Id, w *bufio.Writer, msgs chan Msg) {
	log.Printf("launching new writer id %d", id)
	for m := range msgs {
		if err := ctx.Err(); err == context.Canceled {
			log.Printf("shutting down writer %d", id)
			return
		}

		if id == m.SenderId {
			continue
		}

		//FIXME how to properly handle errors in persistent connections like this?
		// Maybe some retries
		_, err := w.WriteString(m.Content)
		if err != nil {
			log.Printf("error writing msg to connection %d", id)
		}
		err = w.Flush()
		if err != nil {
			log.Printf("error flushing connection %d", id)
		}
	}
}
