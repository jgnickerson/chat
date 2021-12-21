package main

import (
	"bufio"
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

	reads := make(chan net.Conn)
	writes := make(chan net.Conn)
	msgs := make(chan string)
	go func() {
		for {
			c, err := l.Accept()
			if err != nil {
				log.Printf("Error accepting new connetion: %s \n", err)
			}
			reads <- c
			writes <- c
		}
	}()

	go func() {
		writers := make([]*bufio.Writer, 0)
		for {
			select {
			case w := <-writes:
				writers = append(writers, bufio.NewWriter(w))
			case msg := <-msgs:
				for _, w := range writers {
					_, err := w.WriteString(msg)
					if err != nil {
						log.Print("Failed to write")
					}
					err = w.Flush()
					if err != nil {
						log.Println("Failed to flush")
					}
				}
			}
		}
	}()

	go func() {
		for c := range reads {
			go func(c net.Conn) {
				defer func(c net.Conn) {
					err := c.Close()
					if err != nil {
						log.Printf("error closing connection: %s", err)
					}
				}(c)
				r := bufio.NewReader(c)
				if err != nil {
					log.Printf("error %s", err)
				}
				for {
					msg, err := r.ReadString('\n')
					if err == io.EOF {
						break
					}
					if err != nil {
						log.Printf("Error reading string: %s \n", err)
					}
					log.Println(msg)
					msgs <- msg
				}
			}(c)
		}
	}()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, os.Kill)
	<-sig
	log.Println("Shutting Down...")
}
