package protocol

import (
	"encoding/binary"
	"fmt"
)

// SeatOptionsRequest follows SET_SEAT_OPTIONS_PDU::Write at 0x52bc80:
// byte option, byte seat, numeric MechID, byte state. The zero seat in
// local selection/state requests is not a four-byte reserved field.
type SeatOptionsRequest struct {
	Option byte
	Seat   byte
	MechID uint32
	State  byte
}

const (
	SeatOptionMove  byte = 0
	SeatOptionMech  byte = 1
	SeatOptionState byte = 2
)

func ParseSeatOptionsRequest(body []byte) (SeatOptionsRequest, error) {
	if len(body) != 7 {
		return SeatOptionsRequest{}, fmt.Errorf("seat-options request length %d, want 7", len(body))
	}
	request := SeatOptionsRequest{Option: body[0], Seat: body[1], MechID: binary.BigEndian.Uint32(body[2:6]), State: body[6]}
	if request.Option > SeatOptionState {
		return SeatOptionsRequest{}, fmt.Errorf("unknown seat option %d", request.Option)
	}
	return request, nil
}

// ParseReadyStateChange accepts only local ready/unready requests. Other
// native seat states (Mech Bay, browsing, modal UI) are not Ready actions.
func ParseReadyStateChange(body []byte) (bool, error) {
	request, err := ParseSeatOptionsRequest(body)
	if err != nil {
		return false, err
	}
	if request.Option != SeatOptionState || request.Seat != 0 || request.MechID != 0 {
		return false, fmt.Errorf("unsupported ready-state request %x", body)
	}
	if request.State > 1 {
		return false, fmt.Errorf("invalid ready-state value %d", request.State)
	}
	return request.State == 1, nil
}

const (
	CountdownTimerID    uint16 = 0xa9
	CountdownStopped    byte   = 0
	CountdownAutoLaunch byte   = 3
	EnterCombatID       uint16 = 0x80
)

// CountdownTimer matches native writer 0x523250: u32 seconds, byte kind.
// The Mission Room starts its own one-second timer on receipt (0x460a55).
func CountdownTimer(seconds uint32, kind byte) PDU {
	body := make([]byte, 5)
	binary.BigEndian.PutUint32(body, seconds)
	body[4] = kind
	return PDU{ID: CountdownTimerID, Body: body}
}

// EnterCombat matches the native 0x80 message: battle type byte followed by
// the teams-rebalanced flag. Solaris missions use the regular battle type.
func EnterCombat() PDU {
	return PDU{ID: EnterCombatID, Body: []byte{0, 0}}
}
