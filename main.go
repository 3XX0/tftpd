package main

import (
	"github.com/3XX0/tftpd/filesystem"
	"log"
	"net"
	"os"
	"os/signal"
	"sync"
	"time"
)

var fs = filesystem.New()

func gotSignal(c <-chan os.Signal) bool {
	select {
	case <-c:
		return true
	default:
	}
	return false
}

func main() {
	var wg sync.WaitGroup

	buf := make([]byte, 512)
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, os.Kill)

	conn, err := net.ListenUDP("udp", &net.UDPAddr{Port: 69})
	if err != nil {
		log.Fatalf("error listenning on tftpd service (%v)\n", err)
	}
	defer conn.Close()

loop:
	for {
		conn.SetReadDeadline(time.Now().Add(1 * time.Second))
		n, raddr, err := conn.ReadFromUDP(buf)

		if gotSignal(sig) {
			break loop
		}
		if err != nil {
			if !errDeadline(err) {
				log.Printf("could not read (%v)\n", err)
			}
			continue
		}
		pkt, err := newPacket(buf, n)
		if err != nil {
			log.Printf("could not decode packet (%v)\n", err)
			continue
		}

		wg.Add(1)
		go func() {
			ctx := newSessionContext(conn, raddr)
			process(ctx, pkt)
			wg.Done()
		}()
	}

	log.Println("tftpd terminating...")
	wg.Wait()
}
