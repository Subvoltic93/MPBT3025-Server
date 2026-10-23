// Package aries implements the outer transport used by the supplied aries.dll.
// It is independent of BattleTech's inner, big-endian PDU encoding.
package aries

import (
	"encoding/binary"
	"fmt"
	"io"
)

const (
	Data         uint32 = 0
	Redirect     uint32 = 0x03
	Login        uint32 = 0x15
	LoginRequest uint32 = 0x16
	MaxPayload          = 1 << 20
)

// Frame has a 12-byte little-endian header: type, tag, payload length.
type Frame struct {
	Type    uint32
	Tag     uint32
	Payload []byte
}

func Read(r io.Reader) (Frame, error) {
	var header [12]byte
	if _, err := io.ReadFull(r, header[:]); err != nil {
		return Frame{}, err
	}
	f := Frame{Type: binary.LittleEndian.Uint32(header[:4]), Tag: binary.LittleEndian.Uint32(header[4:8])}
	size := binary.LittleEndian.Uint32(header[8:])
	if size > MaxPayload {
		return Frame{}, fmt.Errorf("Aries payload too large: %d", size)
	}
	f.Payload = make([]byte, size)
	_, err := io.ReadFull(r, f.Payload)
	return f, err
}

func Write(w io.Writer, f Frame) error {
	if len(f.Payload) > MaxPayload {
		return fmt.Errorf("Aries payload too large: %d", len(f.Payload))
	}
	data := make([]byte, 12+len(f.Payload))
	binary.LittleEndian.PutUint32(data, f.Type)
	binary.LittleEndian.PutUint32(data[4:], f.Tag)
	binary.LittleEndian.PutUint32(data[8:], uint32(len(f.Payload)))
	copy(data[12:], f.Payload)
	for len(data) > 0 {
		n, err := w.Write(data)
		if err != nil {
			return err
		}
		if n == 0 {
			return io.ErrShortWrite
		}
		data = data[n:]
	}
	return nil
}
