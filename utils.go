package main

import (
	"bytes"
	"encoding/binary"
	"net"
)

type NetShort uint16

func (w *NetShort) read(buf []byte) error {
	r := bytes.NewReader(buf)
	return binary.Read(r, binary.BigEndian, w)
}

func (w *NetShort) write(b *bytes.Buffer) error {
	return binary.Write(b, binary.BigEndian, w)
}

// Parse a NULL terminated string from a given buffer.
func parseString(buf []byte) (int, string) {
	i := bytes.IndexByte(buf, 0)
	if i < 0 {
		return 0, ""
	}
	return i + 1, string(buf[:i])
}

// Returns whether a given error is a network timeout or not.
func errDeadline(err error) bool {
	if e, ok := err.(net.Error); ok && e.Timeout() {
		return true
	}
	return false
}
