package protocol

import (
	"bytes"
	"encoding/binary"
)

// PlayerUpdate is a development pilot using the wire order recovered from the
// client serializers at 0x4fce40 and 0x51b690.
func PlayerUpdate() PDU {
	return PlayerUpdateWithID("Local Pilot", "")
}

// PlayerUpdateFor builds the same recovered player record with a session's
// call sign instead of the bootstrap placeholder.
func PlayerUpdateFor(callSign string) PDU {
	return PlayerUpdateWithID(callSign, callSign)
}

// PlayerUpdateWithID builds a BTPlayerInfo record whose cache key is the
// string PlayerID. Its virtual writer at 0x4f7ff0 delegates to the string
// writer, not the uint32 writer. The profile must use the same identity.
func PlayerUpdateWithID(callSign string, playerID string) PDU {
	var body bytes.Buffer
	putString(&body, playerID) // PlayerID::m_ariesID, a string at +0x04
	body.WriteByte(0)          // base field at +0x21; serialized before the bool
	putBool(&body, false)      // base field at +0x20
	putString(&body, callSign)
	putEmptyContainer(&body) // field at +0x34
	body.WriteByte(0)        // field at +0x3c
	putUint32(&body, 0)      // field at +0x40
	putUint32(&body, 0)      // field at +0x44
	putUint32(&body, 0)      // field at +0x48
	putBool(&body, false)    // field at +0x54
	putString(&body, "")     // field at +0x4c; serialized after +0x54
	return PDU{PlayerUpdateID, body.Bytes()}
}

// OccupantUpdateFor wraps the BTPlayerInfo record used by PLAYER_UPDATE in
// OCCUPANT_UPDATE. Keep this separate from the lobby's OCCUPANT_ARRIVED event:
// the two PDUs have different client-side roster behavior.
func OccupantUpdateFor(callSign string) PDU {
	return OccupantUpdateWithID(callSign, callSign)
}

func OccupantUpdateWithID(callSign string, playerID string) PDU {
	player := PlayerUpdateWithID(callSign, playerID)
	return PDU{OccupantUpdateID, player.Body}
}

// OccupantArrivedFor announces a pilot entering a planet's live occupant list.
// The 0x2b serializer writes the nested BTPlayerInfo object directly, matching
// the player record body used by PLAYER_UPDATE and OCCUPANT_UPDATE.
func OccupantArrivedFor(callSign string) PDU {
	return OccupantArrivedWithID(callSign, callSign)
}

func OccupantArrivedWithID(callSign string, playerID string) PDU {
	player := PlayerUpdateWithID(callSign, playerID)
	return PDU{OccupantArrivedID, player.Body}
}

// OccupantDepartedFor removes the pilot's record from connected clients' live
// occupant lists. The 0x2c serializer has the same nested player-info layout.
func OccupantDepartedFor(callSign string) PDU {
	return OccupantDepartedWithID(callSign, callSign)
}

func OccupantDepartedWithID(callSign string, playerID string) PDU {
	player := PlayerUpdateWithID(callSign, playerID)
	return PDU{OccupantDepartedID, player.Body}
}

// ProfileUpdate is a disposable local profile using the BTProfileInfo wire
// order recovered at 0x4fd8b0 and 0x51c430. The uint32 at +0x94 is serialized
// after +0xbc, so keep the write order aligned with the client serializer.
func ProfileUpdate() PDU {
	return ProfileUpdateWithID("Local Pilot", "")
}

// ProfileUpdateFor supplies the empty-development profile under the pilot's
// persisted call sign.
func ProfileUpdateFor(callSign string) PDU {
	return ProfileUpdateWithID(callSign, callSign)
}

// ProfileUpdateWithID pairs profile data with a BTPlayerInfo cache key.
func ProfileUpdateWithID(callSign string, playerID string) PDU {
	var body bytes.Buffer
	putString(&body, playerID) // PlayerID::m_ariesID, a string at +0x04
	putString(&body, callSign) // base field at +0x20
	putBool(&body, false)      // base field at +0x30
	putEmptyContainer(&body)   // base field at +0x34
	putUint32(&body, 0)        // base field at +0x3c
	putUint32(&body, 0)        // base field at +0x40
	putEmptyContainer(&body)   // base field at +0x48
	putEmptyContainer(&body)   // base field at +0x50
	putEmptyContainer(&body)   // base field at +0x58
	putEmptyContainer(&body)   // base field at +0x64
	putString(&body, "")       // base field at +0x6c
	body.WriteByte(0)          // base field at +0x44; serialized here
	putBool(&body, false)      // base field at +0x60

	putString(&body, "")     // profile field at +0x7c
	putEmptyContainer(&body) // profile field at +0x8c
	body.WriteByte(1)        // profile field at +0x98
	putUint32(&body, 0)      // profile field at +0x9c
	putUint32(&body, 0)      // profile field at +0xa0
	putUint32(&body, 0)      // profile field at +0xa4
	putEmptyContainer(&body) // profile field at +0xa8
	putEmptyContainer(&body) // profile field at +0xb0
	putBool(&body, false)    // profile field at +0xb8
	putUint32(&body, 0)      // profile field at +0xbc
	putUint32(&body, 0)      // profile field at +0x94; serialized after +0xbc
	putEmptyContainer(&body) // profile field at +0xc0
	putString(&body, "")     // profile field m_lastPlanet at +0xd4; keep unset
	body.WriteByte(0)        // profile field at +0xdc
	return PDU{ProfileUpdateID, body.Bytes()}
}

func putEmptyContainer(w *bytes.Buffer) { putUint32(w, 0) }

func putBool(w *bytes.Buffer, value bool) {
	if value {
		w.WriteByte(1)
		return
	}
	w.WriteByte(0)
}

func putUint32(w *bytes.Buffer, value uint32) { binary.Write(w, binary.BigEndian, value) }
func putString(w *bytes.Buffer, value string) { putUint32(w, uint32(len(value))); w.WriteString(value) }
