package main

import (
	"bytes"
	"encoding"
	"errors"
	"io"
	"reflect"
)

var (
	ErrPktMalformed   = errors.New("malformed packet")
	ErrPktUnsupported = errors.New("unsupported packet")
)

// Operation codes
const (
	_ NetShort = iota
	ReadReq
	WriteReq
	Data
	Ack
	Error
)

// Error codes
const (
	Undefined NetShort = iota
	FileNotFound
	AccessViolation
	DiskFull
	IllegalOp
	UnknownTID
	FileExists
	NoSuchUser
)

type Packet interface {
	OpCode() NetShort
	encoding.BinaryMarshaler
	encoding.BinaryUnmarshaler
}

type DataPacket struct {
	opcode NetShort
	block  NetShort
	data   []byte
}

type AckPacket struct {
	opcode NetShort
	block  NetShort
}

type ErrorPacket struct {
	opcode  NetShort
	errcode NetShort
	msg     string
}

type reqPacket struct {
	opcode   NetShort
	filepath string
	mode     string
}

type WriteReqPacket struct{ reqPacket }
type ReadReqPacket struct{ reqPacket }

var packets = map[NetShort]Packet{
	ReadReq:  (*ReadReqPacket)(nil),
	WriteReq: (*WriteReqPacket)(nil),
	Data:     (*DataPacket)(nil),
	Ack:      (*AckPacket)(nil),
	Error:    (*ErrorPacket)(nil),
}

// Creates a new packet from a given buffer.
func newPacket(buf []byte, n int) (Packet, error) {
	var opcode NetShort

	if n < 4 { // minimum mandatory by the RFC 1350
		return nil, io.ErrShortBuffer
	}
	if err := opcode.read(buf[:2]); err != nil {
		return nil, err
	}
	if p := packets[opcode]; p != nil {
		// reflect the packet type according to the opcode
		t := reflect.TypeOf(p).Elem()
		p = reflect.New(t).Interface().(Packet)
		if err := p.UnmarshalBinary(buf[2:n]); err != nil {
			return nil, err
		}
		return p, nil
	}
	return nil, ErrPktUnsupported
}

func newData(block NetShort, data []byte) *DataPacket { return &DataPacket{Data, block, data} }
func newAck(block NetShort) *AckPacket                { return &AckPacket{Ack, block} }
func newError(code NetShort, msg string) *ErrorPacket { return &ErrorPacket{Error, code, msg} }

func (p *WriteReqPacket) OpCode() NetShort { return WriteReq }
func (p *ReadReqPacket) OpCode() NetShort  { return ReadReq }
func (p *DataPacket) OpCode() NetShort     { return Data }
func (p *AckPacket) OpCode() NetShort      { return Ack }
func (p *ErrorPacket) OpCode() NetShort    { return Error }

func (p *reqPacket) MarshalBinary() ([]byte, error) {
	var b bytes.Buffer

	b.Grow(2 + len(p.filepath) + 1 + len(p.mode) + 1)
	if err := p.opcode.write(&b); err != nil {
		return nil, err
	}
	b.WriteString(p.filepath)
	b.WriteByte(0)
	b.WriteString(p.mode)
	b.WriteByte(0)
	return b.Bytes(), nil
}

func (p *reqPacket) UnmarshalBinary(buf []byte) error {
	var n int

	n, p.filepath = parseString(buf)
	if n == 0 {
		return ErrPktMalformed
	}
	n, p.mode = parseString(buf[n:])
	if n == 0 {
		return ErrPktMalformed
	}
	return nil
}

func (p *DataPacket) MarshalBinary() ([]byte, error) {
	var b bytes.Buffer

	b.Grow(2 + 2 + len(p.data))
	if err := p.opcode.write(&b); err != nil {
		return nil, err
	}
	if err := p.block.write(&b); err != nil {
		return nil, err
	}
	b.Write(p.data)
	return b.Bytes(), nil
}

func (p *DataPacket) UnmarshalBinary(buf []byte) error {
	if err := p.block.read(buf[:2]); err != nil {
		return err
	}
	p.data = buf[2:]
	return nil
}

func (p *AckPacket) MarshalBinary() ([]byte, error) {
	var b bytes.Buffer

	b.Grow(2 + 2)
	if err := p.opcode.write(&b); err != nil {
		return nil, err
	}
	if err := p.block.write(&b); err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}

func (p *AckPacket) UnmarshalBinary(buf []byte) error {
	if err := p.block.read(buf[:2]); err != nil {
		return err
	}
	return nil
}

func (p *ErrorPacket) MarshalBinary() ([]byte, error) {
	var b bytes.Buffer

	b.Grow(2 + 2 + len(p.msg) + 1)
	if err := p.opcode.write(&b); err != nil {
		return nil, err
	}
	if err := p.errcode.write(&b); err != nil {
		return nil, err
	}
	b.WriteString(p.msg)
	b.WriteByte(0)
	return b.Bytes(), nil
}

func (p *ErrorPacket) UnmarshalBinary(buf []byte) error {
	var n int

	if err := p.errcode.read(buf[:2]); err != nil {
		return err
	}
	n, p.msg = parseString(buf[2:])
	if n == 0 {
		return ErrPktMalformed
	}
	return nil
}
