package protocol

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"unicode/utf8"
)

// RoomID is the composite serialized by RoomID::Write at 0x4f8710:
// a room number followed by a host-name string. It is not a plain string.
type RoomID struct {
	Number uint32
	Host   string
}

func putRoomID(w *bytes.Buffer, id RoomID) {
	putUint32(w, id.Number)
	putString(w, id.Host)
}

// RoomInfo follows RoomInfo::Write (0x5018b0) and BTRoomInfo::Write
// (0x51f1f0). Zero optional fields represent a plain planet/city lobby.
type RoomInfo struct {
	ID                RoomID
	Name              string
	OwnerID           string
	Stage             RoomID
	Occupancy         uint32
	Capacity          uint32
	Type              byte
	Parent            RoomID
	TotalOccupancy    uint16
	AllowPlayerRooms  bool
	Sanctioned        bool
	MinRank           byte
	MaxRank           byte
	Description       string
	PlanetID          uint32
	DestroyEmpty      bool
	Visible           bool
	AttackerSeats     byte
	DefenderSeats     byte
	OpenAttackerSeats byte
	OpenDefenderSeats byte
}

// RoomUpdate is the server push that replaces the client's current room.
// Its decoder at 0x505f50 reads reason, occupant-list-follows, then RoomInfo.
// The following roster messages must be supplied when occupantsFollow is true.
func RoomUpdate(reason uint32, occupantsFollow bool, room RoomInfo) PDU {
	var body bytes.Buffer
	putUint32(&body, reason)
	putBool(&body, occupantsFollow)
	putRoomInfo(&body, room)
	return PDU{ID: RoomUpdateID, Body: body.Bytes()}
}

// UpdateOccupants is the full roster expected after occupants-follow in 0x3e.
// Writer0x5069d0: request status u32, BTRoomInfo, PlayerInfoList. The list
// writer0x512250 uses a uint16 count, then virtual BTPlayerInfo records.
func UpdateOccupants(room RoomInfo, callSigns []string) PDU {
	var body bytes.Buffer
	putUint32(&body, 0)
	putRoomInfo(&body, room)
	binary.Write(&body, binary.BigEndian, uint16(len(callSigns)))
	for _, name := range callSigns {
		body.Write(PlayerUpdateFor(name).Body)
	}
	return PDU{ID: 0x3b, Body: body.Bytes()}
}

func putRoomInfo(body *bytes.Buffer, room RoomInfo) {
	putRoomID(body, room.ID)
	putString(body, room.Name)
	putBool(body, false) // name locked, RoomInfo +0x38
	putString(body, room.OwnerID)
	putRoomID(body, room.Stage)
	putUint32(body, room.Occupancy)
	putUint32(body, room.Capacity)
	putBool(body, false) // password required, RoomInfo +0x30
	body.WriteByte(room.Type)
	putRoomID(body, room.Parent)
	var total [2]byte
	binary.BigEndian.PutUint16(total[:], room.TotalOccupancy)
	body.Write(total[:])
	putBool(body, room.AllowPlayerRooms)
	putBool(body, room.Sanctioned)
	body.WriteByte(room.MinRank)
	body.WriteByte(room.MaxRank)
	putString(body, room.Description)
	putUint32(body, room.PlanetID) // numeric wrapper, not a string
	putBool(body, room.DestroyEmpty)
	putBool(body, room.Visible)
	putBool(body, false) // in combat
	body.Write([]byte{room.AttackerSeats, room.DefenderSeats, room.OpenAttackerSeats, room.OpenDefenderSeats, 0, 0, 0})
}

// ListRoomsResponseWith uses the recovered 0x27 writer at 0x509f70:
// result, reason string, count, and native RoomInfo records. This is separate
// from the unrelated 0x97 ready-room collection format.
func ListRoomsResponseWith(rooms []RoomInfo) PDU {
	var body bytes.Buffer
	putUint32(&body, 0)
	putString(&body, "")
	putUint32(&body, uint32(len(rooms)))
	for _, room := range rooms {
		putRoomInfo(&body, room)
	}
	return PDU{ID: ListRoomsResponseID, Body: body.Bytes()}
}

// EmptyReadyRoomAdded announces one unoccupied mission. Native writer
// 0x527a50 writes BTRoomInfo followed by the seat map; 0x4e65d0 writes
// that map's count first. Occupied-seat records require their own codec.
func EmptyReadyRoomAdded(room RoomInfo) PDU {
	var body bytes.Buffer
	putRoomInfo(&body, room)
	putUint32(&body, 0)
	return PDU{ID: ReadyRoomAddedID, Body: body.Bytes()}
}

// ParseJoinRoomRequest matches native JOIN_ROOM writer 0x50ac00.
func ParseJoinRoomRequest(body []byte) (RoomID, string, error) {
	var id RoomID
	if len(body) < 12 {
		return id, "", fmt.Errorf("short JOIN_ROOM")
	}
	id.Number = binary.BigEndian.Uint32(body)
	n := uint64(binary.BigEndian.Uint32(body[4:]))
	if n > uint64(len(body)-12) {
		return id, "", fmt.Errorf("invalid room host length")
	}
	id.Host = string(body[8 : 8+n])
	rest := body[8+n:]
	m := uint64(binary.BigEndian.Uint32(rest))
	if m != uint64(len(rest)-4) || !utf8.ValidString(id.Host) || !utf8.Valid(rest[4:]) {
		return id, "", fmt.Errorf("invalid room password")
	}
	return id, string(rest[4:]), nil
}

// SeatLite is the staging list's CombatSeatLite, not CombatSeatInfo.
// The map decoder 0x4e66d0 constructs the lite type at 0x516880.
type SeatLite struct {
	Number, Team byte
	CallSign     string
	House, Rank  uint32
	Badge        byte
}

func putSeatLite(body *bytes.Buffer, seat SeatLite) {
	body.WriteByte(seat.Number)
	body.WriteByte(seat.Team)
	putString(body, seat.CallSign)
	putUint32(body, seat.House)
	putUint32(body, seat.Rank)
	body.WriteByte(seat.Badge)
}
func ReadyRoomAdded(room RoomInfo, seats []SeatLite) PDU {
	var body bytes.Buffer
	putRoomInfo(&body, room)
	putUint32(&body, uint32(len(seats)))
	for _, seat := range seats {
		putSeatLite(&body, seat)
	}
	return PDU{ID: ReadyRoomAddedID, Body: body.Bytes()}
}

// ReadyRoomUpdate uses the same BTRoomInfo/seat-map payload as 0x99.
// Native descriptor 0x5940c0 is 0x98; writer 0x5280b0 mirrors 0x527a50.
func ReadyRoomUpdate(room RoomInfo, seats []SeatLite) PDU {
	p := ReadyRoomAdded(room, seats)
	p.ID = 0x98
	return p
}

// ReadyRoomRemoved carries only RoomID (native writer 0x5276a0).
func ReadyRoomRemoved(room RoomID) PDU {
	var body bytes.Buffer
	putRoomID(&body, room)
	return PDU{ID: 0x9a, Body: body.Bytes()}
}

func JoinRoomFailure(reason string) PDU {
	var body bytes.Buffer
	putUint32(&body, 1)
	putString(&body, reason)
	return PDU{ID: 0x22, Body: body.Bytes()}
}

// CombatSeat follows CombatSeatInfo::Write at 0x517150.
type CombatSeat struct {
	SeatLite
	WeightClass byte
	OccupantID  string
	MechID      uint32
	State       byte
}

const (
	CombatSeatStateNotReady      byte = 0
	CombatSeatStateReady         byte = 1
	CombatSeatStateSelectingMech byte = 2
)

func putCombatSeat(body *bytes.Buffer, seat CombatSeat) {
	putSeatLite(body, seat.SeatLite)
	body.WriteByte(seat.WeightClass)
	putString(body, seat.OccupantID)
	putUint32(body, seat.MechID)
	body.WriteByte(seat.State)
	body.Write(make([]byte, 25)) // four score uint32s, two shot uint16s, kills byte, total uint32
}

func putCombatInfo(body *bytes.Buffer, room RoomID, planet, mapID uint32, climate byte, seats []CombatSeat) {
	putRoomID(body, room)
	putUint32(body, 0)      // native +0x1c numeric wrapper, default value
	putUint32(body, planet) // PlanetID at +0x24, value +0x28
	putUint32(body, mapID)  // MapID at +0x2c, value +0x30
	putUint32(body, 0)      // terrain seed at +0x34
	body.WriteByte(climate) // enum at +0x38; not a uint32
	putBool(body, false)
	putBool(body, true)
	putUint32(body, uint32(len(seats)))
	for _, seat := range seats {
		putCombatSeat(body, seat)
	}
}

// CurrentCombat uses 0x9e and CombatInfo::Write at 0x514ff0. The combat
// manager handles it at 0x4d8696, replacing its +0x148 snapshot; 0x8b
// instead replaces the +0xe4 snapshot at 0x4d8710.
func CurrentCombat(room RoomID, planet, mapID uint32, climate byte, seats []CombatSeat) PDU {
	var body bytes.Buffer
	putCombatInfo(&body, room, planet, mapID, climate, seats)
	return PDU{ID: 0x9e, Body: body.Bytes()}
}

// CombatConfigurationUpdate carries CombatInfo followed by its reason enum.
// Unlike 0x9e, 0x8b reaches the native combat-configuration receive handler.
func CombatConfigurationUpdate(room RoomID, planet, mapID uint32, climate byte, seats []CombatSeat, reason uint32) PDU {
	var body bytes.Buffer
	putCombatInfo(&body, room, planet, mapID, climate, seats)
	putUint32(&body, reason)
	return PDU{ID: CombatConfigurationUpdateID, Body: body.Bytes()}
}

func SeatOptionsUpdate(seat CombatSeat, reason uint32) PDU {
	var body bytes.Buffer
	putCombatSeat(&body, seat)
	putUint32(&body, reason)
	return PDU{ID: SeatOptionsUpdateID, Body: body.Bytes()}
}
