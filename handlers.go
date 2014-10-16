package main

import (
	"fmt"
	"io"
	"log"
	"net"
	"path"
	"strings"
	"time"
)

const dallyDelay = 3 // 3 seconds

type PacketHandler func(*SessionContext, Packet) bool

var handlers map[NetShort]PacketHandler

func init() {
	// XXX workaround to avoid variable initialization loop
	handlers = map[NetShort]PacketHandler{
		ReadReq:  handleReadReq,
		WriteReq: handleWriteReq,
		Data:     handleData,
		Ack:      handleAck,
		//Error:    handleError,
	}
}

func connTimeout(c <-chan struct{}) bool {
	select {
	case <-c:
		return true
	default:
	}
	return false
}

func serveReq(ctx *SessionContext, dally int) error {
	buf := make([]byte, 2+2+512)

	for i, done := 0, false; !done || i < dally; {
		ctx.conn.SetReadDeadline(time.Now().Add(1 * time.Second))
		n, addr, err := ctx.conn.ReadFromUDP(buf)

		if connTimeout(ctx.timeout) {
			return fmt.Errorf("%s: connection timeout", ctx.raddr.String())
		}
		if err != nil {
			if errDeadline(err) {
				if done {
					i++
				}
			} else {
				log.Printf("could not read (%v)\n", err)
			}
			continue
		}
		i = 0
		if !addr.IP.Equal(ctx.raddr.IP) || addr.Port != ctx.raddr.Port {
			send(ctx, newError(UnknownTID, ""), true)
			continue
		}
		pkt, err := newPacket(buf, n)
		if err != nil {
			log.Printf("could not decode packet (%v)\n", err)
			continue
		}
		done = process(ctx, pkt)
	}
	return nil
}

func handleWriteReq(ctx *SessionContext, pkt Packet) (_ bool) {
	var err error

	ctx.state = WRQ
	ctx.conn, err = net.ListenUDP("udp", new(net.UDPAddr))
	if err != nil {
		log.Printf("could not bind (%v)\n", err)
		return
	}
	defer ctx.conn.Close()

	p := pkt.(*WriteReqPacket)
	if strings.ToLower(p.mode) != "octet" {
		send(ctx, newError(Undefined, "unsupported mode of operation"), true)
		return
	}

	f := path.Base(p.filepath)
	log.Printf("write request from %s: put %s", ctx.raddr.String(), f)

	ctx.file = fs.CreateMemoryFile(f)
	if ok := send(ctx, newAck(0), false); !ok {
		return
	}
	if err := serveReq(ctx, dallyDelay); err != nil {
		log.Println(err)
		return
	}
	fs.Save(ctx.file)

	return
}

func handleData(ctx *SessionContext, pkt Packet) (_ bool) {
	p := pkt.(*DataPacket)

	if ctx.block != p.block {
		return // bad block
	}
	off := (int64(ctx.block) - 1) * 512
	if _, err := ctx.file.WriteAt(p.data, off); err != nil {
		log.Printf("failed to write to filesystem (%v)\n", err)
		return
	}
	confirmPreviousPktSent(ctx)
	if len(p.data) < 512 {
		send(ctx, newAck(ctx.block), true)
		return true // last block
	}
	if ok := send(ctx, newAck(ctx.block), false); !ok {
		return
	}

	ctx.block++
	return
}

func readBlock(ctx *SessionContext, block NetShort) (Packet, error) {
	buf := make([]byte, 512)

	off := (int64(block) - 1) * 512
	n, err := ctx.file.ReadAt(buf, off)
	if err != nil {
		if err != io.EOF {
			return nil, fmt.Errorf("failed to read from filesystem (%v)", err)
		} else if ctx.lastBlock {
			return nil, nil
		}
	}
	if n < 512 {
		ctx.lastBlock = true
	}
	return newData(block, buf[:n]), nil
}

func getDataBlock(ctx *SessionContext) (Packet, error)     { return readBlock(ctx, ctx.block) }
func getNextDataBlock(ctx *SessionContext) (Packet, error) { return readBlock(ctx, ctx.block+1) }

func handleReadReq(ctx *SessionContext, pkt Packet) (_ bool) {
	var err error

	ctx.state = RRQ
	ctx.conn, err = net.ListenUDP("udp", new(net.UDPAddr))
	if err != nil {
		log.Printf("could not bind (%v)\n", err)
		return
	}
	defer ctx.conn.Close()

	p := pkt.(*ReadReqPacket)
	if strings.ToLower(p.mode) != "octet" {
		send(ctx, newError(Undefined, "unsupported mode of operation"), true)
		return
	}

	f := path.Base(p.filepath)
	log.Printf("read request from %s: get %s", ctx.raddr.String(), f)

	ctx.file, err = fs.Open(f)
	if err != nil {
		send(ctx, newError(FileNotFound, ""), true)
		return
	}
	b, err := getDataBlock(ctx)
	if err != nil {
		log.Println(err)
		return
	}
	if ok := send(ctx, b, false); !ok {
		return
	}
	if err := serveReq(ctx, 0); err != nil {
		log.Println(err)
		return
	}
	ctx.file.Close()

	return
}

func handleAck(ctx *SessionContext, pkt Packet) (_ bool) {
	p := pkt.(*AckPacket)

	if ctx.block != p.block {
		return // bad block
	}
	b, err := getNextDataBlock(ctx)
	if err != nil {
		log.Println(err)
		return
	}
	confirmPreviousPktSent(ctx)
	if b == nil {
		return true // last block ack
	}
	if ok := send(ctx, b, false); !ok {
		return
	}

	ctx.block++
	return
}

/*
func handleError(ctx *SessionContext, pkt Packet) (_ bool) {
	p := pkt.(*ErrorPacket)

	confirmPreviousPktSent(ctx)
	_ = fmt.Errorf("error encountered from %s: (code=%v) %s", ctx.raddr.String(), p.errcode, p.msg)
	return
}
*/
