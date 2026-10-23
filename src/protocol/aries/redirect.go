package aries

import "fmt"

const redirectFieldSize = 40

// RedirectFrame builds the 120-byte ARIES redirect shape documented by the
// related MPBT: Solaris emulator. Its use for the 3025 client is still an
// experiment; keep the 3025 lobby's normal behavior unchanged by default.
func RedirectFrame(address, internetAddress, password string) (Frame, error) {
	values := []string{address, internetAddress, password}
	payload := make([]byte, redirectFieldSize*len(values))
	for index, value := range values {
		if len(value) >= redirectFieldSize {
			return Frame{}, fmt.Errorf("redirect field %d is %d bytes; maximum is %d", index+1, len(value), redirectFieldSize-1)
		}
		copy(payload[index*redirectFieldSize:], value)
	}
	return Frame{Type: Redirect, Payload: payload}, nil
}
