package main

import (
	"github.com/3XX0/tftpd/filesystem"
	"log"
	"net"
	"time"
)

const (
	retransmissionDelay   = 500 * time.Millisecond
	retransmissionRetries = 20 // 10 seconds timeout
)

type SessionState uint8

const (
	INI SessionState = 0x01
	RRQ              = 0x02
	WRQ              = 0x04
)

type SessionContext struct {
	state     SessionState
	conn      *net.UDPConn
	raddr     *net.UDPAddr
	ack       chan bool
	timeout   chan struct{}
	block     NetShort
	file      filesystem.File
	activeTx  bool
	lastBlock bool
}

// Used to determines which operations are allowed depending on
// the state of the current session.
var transitionMap = map[NetShort]SessionState{
	ReadReq:  INI,
	WriteReq: INI,
	Data:     WRQ,
	Ack:      RRQ,
	//Error:    WRQ | RRQ,
}

// Crates a new session for a file transfert.
func newSessionContext(conn *net.UDPConn, raddr *net.UDPAddr) *SessionContext {
	return &SessionContext{
		state:   INI,
		conn:    conn,
		raddr:   raddr,
		ack:     make(chan bool),
		timeout: make(chan struct{}),
		block:   1,
	}
}

// Process a given packet by calling the appropriate handler.
func process(ctx *SessionContext, pkt Packet) bool {
	opcode := pkt.OpCode()
	if t := transitionMap[opcode]; t&ctx.state == 0 {
		send(ctx, newError(IllegalOp, ""), true)
		return false
	}
	return handlers[opcode](ctx, pkt)
}

// Send a packet in the current session, retransmitting it if necessary.
func send(ctx *SessionContext, pkt Packet, once bool) bool {
	b, err := pkt.MarshalBinary()
	if err != nil {
		log.Printf("could not encode packet (%v)\n", err)
		return false
	}
	if _, err := ctx.conn.WriteToUDP(b, ctx.raddr); err != nil {
		log.Printf("could not write (%v)\n", err)
		return false
	}
	if once {
		return true
	}

	go func() {
		t := time.NewTicker(retransmissionDelay)
		for i := 0; i < retransmissionRetries; i++ {
			select {
			case <-ctx.ack:
				t.Stop()
				return
			case <-t.C:
			}
			if _, err := ctx.conn.WriteToUDP(b, ctx.raddr); err != nil {
				log.Printf("could not write (%v)\n", err)
			}
		}
		t.Stop()
		close(ctx.timeout) // connection timeout
	}()

	ctx.activeTx = true
	return true
}

// Confirm a previous send operation, thus stopping retransmissions.
func confirmPreviousPktSent(ctx *SessionContext) {
	if ctx.activeTx {
		select {
		case ctx.ack <- true:
		case <-ctx.timeout:
		}
		ctx.activeTx = false
	}
}
