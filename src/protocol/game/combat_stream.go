package protocol

import (
	"encoding/binary"
	"fmt"
)

const CombatDataID uint16 = 0x81

// CombatData wraps an already serialized native combat command stream.
// BT3025.exe:0x520130 writes a u32 length followed by that many raw bytes;
// 0x4d8610 passes that exact length to BTCombat.dll's ReceiveCombatData.
// The native receive preprocessor (0x690751c0) removes both ESC delimiters
// and a terminal NUL, so the NUL must be included in the counted data.
// Command payloads must be constructed separately before calling this function.
func CombatData(commandStream []byte) PDU {
	inner := make([]byte, 1, len(commandStream)+1)
	inner[0] = '!'
	inner = append(inner, commandStream...)
	checksum := combatChecksum(inner, true)

	// ESC + inner + three checksum digits + ESC + NUL.
	dataLen := len(inner) + 6
	body := make([]byte, 4, 4+dataLen)
	binary.BigEndian.PutUint32(body, uint32(dataLen))
	body = append(body, 0x1b)
	body = append(body, inner...)
	body = append(body, byte(checksum/(85*85))+'!', byte(checksum/85%85)+'!', byte(checksum%85)+'!')
	body = append(body, 0x1b, 0)
	return PDU{ID: CombatDataID, Body: body}
}

// CombatReadyCommand is the zero-payload command whose visitor calls the
// engine's ReadyForCombat. Terrain and Mech commands must precede it.
func CombatReadyCommand() []byte { return []byte{'_'} }

// CombatPeerPosition maps the shared fields of the client's native position
// command (key 6, writer 0x6906f700) into the server position command
// (key 0x41, writer 0x6906f350). The entity ID comes from the authenticated
// room roster; the client command has no sender ID on the wire.
func CombatPeerPosition(entityID byte, clientCommand []byte) ([]byte, error) {
	if entityID >= 85 || len(clientCommand) != 37 || clientCommand[0] != '\x27' {
		return nil, fmt.Errorf("invalid native client position command or entity %d", entityID)
	}
	for _, digit := range clientCommand[1:24] {
		if digit < '!' || digit > 'u' {
			return nil, fmt.Errorf("invalid native position digit")
		}
	}
	command := make([]byte, 0, 25)
	command = append(command, 'b', entityID+'!')
	command = append(command, clientCommand[1:24]...)
	return command, nil
}

// ClientCombatPosition extracts the regular position command. A weapon shot
// appends a 36-byte native fire command (key 0x0a) to the same client frame;
// its fields still need combat simulation, but the leading position remains
// useful to the peer.
func ClientCombatPosition(commands []byte) ([]byte, bool) {
	if len(commands) == 37 && commands[0] == '\x27' {
		return commands, true
	}
	if len(commands) == 73 && commands[0] == '\x27' && commands[37] == '+' {
		return commands[:37], true
	}
	return nil, false
}

// CombatPeerFire converts the client's native fire report (key 0x0a) into
// the corresponding server command (key 0x44). The server writer adds the
// authenticated firing entity and omits the client's final one-byte field.
func CombatPeerFire(entityID byte, clientFire []byte) ([]byte, error) {
	if entityID >= 85 || len(clientFire) != 36 || clientFire[0] != '+' {
		return nil, fmt.Errorf("invalid native client fire command or entity %d", entityID)
	}
	for _, digit := range clientFire[1:] {
		if digit < '!' || digit > 'u' {
			return nil, fmt.Errorf("invalid native fire digit")
		}
	}
	command := make([]byte, 0, 36)
	command = append(command, 'e', entityID+'!')
	command = append(command, clientFire[1:35]...)
	return command, nil
}

// ParseClientCombatData verifies the DLL's outbound stream. Unlike server
// input, native client frames do not count a terminal NUL in their raw data.
func ParseClientCombatData(body []byte) ([]byte, error) {
	if len(body) < 4 || uint64(binary.BigEndian.Uint32(body)) != uint64(len(body)-4) {
		return nil, fmt.Errorf("combat data length does not match its body")
	}
	frame := body[4:]
	if len(frame) > 0 && frame[len(frame)-1] == 0 {
		frame = frame[:len(frame)-1]
	}
	if len(frame) < 7 || frame[0] != 0x1b || frame[len(frame)-1] != 0x1b || frame[1] != '!' {
		return nil, fmt.Errorf("invalid native combat framing")
	}
	inner := frame[1 : len(frame)-4]
	var checksum uint32
	for _, digit := range frame[len(frame)-4 : len(frame)-1] {
		if digit < '!' || digit > 'u' {
			return nil, fmt.Errorf("invalid native combat checksum digit")
		}
		checksum = checksum*85 + uint32(digit-'!')
	}
	if combatChecksum(inner, false) != checksum {
		return nil, fmt.Errorf("native combat checksum mismatch")
	}
	return inner[1:], nil
}

// combatChecksum follows the original instructions at BTCombat.dll:0x69075200,
// including the final folding steps. The context's direction flag selects
// 0xa5c45 for server input and 0xc4545 for client output.
func combatChecksum(data []byte, fromServer bool) uint32 {
	value := uint32(0xc4545)
	if fromServer {
		value = 0xa5c45
	}
	step := func(value uint32) uint32 {
		value <<= 1
		if value&0x80000 != 0 {
			value = value&0x7fffe | 1
		}
		return value
	}
	for _, b := range data {
		// The DLL uses MOVSX for the source byte.
		value = step(value) ^ uint32(int32(int8(b)))
	}
	folded := step(value)
	folded = step(value&0xff ^ folded)
	folded = step(value>>8&0xff ^ folded)
	return value>>16&7 ^ folded
}
