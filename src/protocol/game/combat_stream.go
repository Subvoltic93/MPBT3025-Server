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

// ClientCombatBatch is the supported part of a native client stream: an
// optional leading position followed by zero or more weapon fire reports.
type ClientCombatBatch struct {
	Position []byte
	Fires    [][]byte
}

// ParseClientCombatCommands accepts complete supported commands only. Live
// traffic contains both standalone fire and position-plus-fire frames.
// Rejecting unknown/truncated tails prevents forwarding a partial batch.
func ParseClientCombatCommands(commands []byte) (ClientCombatBatch, error) {
	var batch ClientCombatBatch
	if len(commands) == 0 {
		return batch, fmt.Errorf("empty native combat command stream")
	}
	if commands[0] == '\x27' {
		if len(commands) < 37 {
			return ClientCombatBatch{}, fmt.Errorf("truncated native position command")
		}
		for _, digit := range commands[1:37] {
			if digit < '!' || digit > 'u' {
				return ClientCombatBatch{}, fmt.Errorf("invalid native position digit")
			}
		}
		batch.Position, commands = commands[:37], commands[37:]
	}
	for len(commands) > 0 {
		if len(commands) < 36 {
			return ClientCombatBatch{}, fmt.Errorf("truncated native fire command")
		}
		if _, err := ParseClientCombatFire(commands[:36]); err != nil {
			return ClientCombatBatch{}, err
		}
		batch.Fires = append(batch.Fires, commands[:36])
		commands = commands[36:]
	}
	return batch, nil
}

// ClientCombatPosition extracts a position for the native preparation barrier;
// a standalone weapon report cannot establish that the client is prepared.
func ClientCombatPosition(commands []byte) ([]byte, bool) {
	batch, err := ParseClientCombatCommands(commands)
	return batch.Position, err == nil && batch.Position != nil
}

// CombatPeerFire converts the client's native fire report (key 0x0a) into
// the corresponding server command (key 0x44). The server writer adds the
// authenticated firing entity and omits the client's final one-byte field.
func CombatPeerFire(entityID byte, clientFire []byte) ([]byte, error) {
	if entityID >= 85 {
		return nil, fmt.Errorf("invalid native client fire command or entity %d", entityID)
	}
	if _, err := ParseClientCombatFire(clientFire); err != nil {
		return nil, err
	}
	command := make([]byte, 0, 36)
	command = append(command, 'e', entityID+'!')
	command = append(command, clientFire[1:35]...)
	return command, nil
}

// CombatFireSelection identifies the references in a native client fire
// report. TargetEntity is -1 for an untargeted shot, otherwise a zero-based
// collision entity ID, copied from HitReturn rather than HUD selection.
// It remains a client report, not an independently validated hit.
type CombatFireSelection struct {
	WeaponSlot   byte
	TargetEntity int
	// HitPart is the reported collision part, minus one; -1 means none.
	// It is an armor index only when TargetEntity names a Mech.
	HitPart int
}

// ParseClientCombatFire follows writer 0x69070c20. Its first two payload
// digits become server object fields +0x31 (weapon slot) and +0x32 (target).
// Visitor 0x69073770 subtracts one from the target before looking up a Mech.
func ParseClientCombatFire(command []byte) (CombatFireSelection, error) {
	if len(command) != 36 || command[0] != '+' {
		return CombatFireSelection{}, fmt.Errorf("invalid native client fire command")
	}
	for _, digit := range command[1:] {
		if digit < '!' || digit > 'u' {
			return CombatFireSelection{}, fmt.Errorf("invalid native fire digit")
		}
	}
	part := 0
	for _, digit := range command[3:6] {
		part = part*85 + int(digit-'!')
	}
	return CombatFireSelection{WeaponSlot: command[1] - '!', TargetEntity: int(command[2]-'!') - 1, HitPart: part - 1}, nil
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
