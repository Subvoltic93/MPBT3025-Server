// Package protocol contains BattleTech 3025's game messages, inside Aries data.
package protocol

import (
	"encoding/binary"
	"fmt"
	"unicode/utf8"
)

const (
	ClientBufferSize        = 0x4000
	ChatMessageID    uint16 = 0x08
	// Chat channel 1 is not present in the user's observed map-chat trace, but
	// the PDU stores channel as a byte; observed All=0 and House=2 bracket the
	// manual's Team channel, making 1 the working enum value until a live room
	// capture can confirm it.
	ChatChannelAll              byte   = 0x00
	ChatChannelTeam             byte   = 0x01
	ChatChannelHouse            byte   = 0x02
	ClientOnline                uint16 = 0x09
	PlayerUpdateID              uint16 = 0x3c
	ProfileUpdateID             uint16 = 0x3d
	RoomUpdateID                uint16 = 0x3e
	SetAcceptAlerts             uint16 = 0x31
	SetIgnoreList               uint16 = 0x33
	SetInvincibility            uint16 = 0x35
	SetInvisibility             uint16 = 0x37
	SetAcceptFlashes            uint16 = 0x43
	SplitBufferPDU              uint16 = 0x47
	JoinRoom                    uint16 = 0x23
	ListRooms                   uint16 = 0x26
	ListRoomsResponseID         uint16 = 0x27
	OccupantArrivedID           uint16 = 0x2b
	OccupantDepartedID          uint16 = 0x2c
	HostOnline                  uint16 = 0x1d
	StaticConfigResponse        uint16 = 0xa4
	StaticConfigUpdate          uint16 = 0x86
	CreateCharacter             uint16 = 0x87
	CreateCharacterReply        uint16 = 0x88
	ExitRoom                    uint16 = 0x17
	CombatConfigurationUpdateID uint16 = 0x8b
	ReadyStateChangeID          uint16 = 0x8c
	SeatOptionsUpdateID         uint16 = 0x8e
	CombatSummary               uint16 = 0x90
	JoinPlanet                  uint16 = 0x91
	JoinPlanetFailed            uint16 = 0x92
	OccupantUpdateID            uint16 = 0x94
	ReadyRoomList               uint16 = 0x97
	ReadyRoomAddedID            uint16 = 0x99
	RequestPlanetStates         uint16 = 0xa7
	PlanetStateList             uint16 = 0xa8
	RequestLobbyRooms           uint16 = 0xad
)

type PDU struct {
	ID   uint16
	Body []byte
}

// Encode includes the six-byte header in the big-endian PDU length.
func Encode(p PDU) []byte {
	data := make([]byte, 6+len(p.Body))
	binary.BigEndian.PutUint16(data, p.ID)
	binary.BigEndian.PutUint32(data[2:], uint32(len(data)))
	copy(data[6:], p.Body)
	return data
}

// FragmentPDU wraps an oversized encoded PDU in SPLIT_BUFFER_PDU messages.
// The inner byte stream includes the original six-byte PDU header. The client
// serializer at BT3025.exe:0x5027a0 and decoder at 0x502740 confirm each
// wrapper contains a final-fragment bool, a uint32 byte length, and raw bytes.
// Those routines establish the fragment format, but not how the client joins
// fragments before dispatching the enclosed PDU.
func FragmentPDU(p PDU, maxWireSize int) ([]PDU, error) {
	const splitHeaderSize = 6 + 1 + 4 // PDU header, m_lastInSet, m_dataLen

	if maxWireSize < splitHeaderSize {
		return nil, fmt.Errorf("maximum PDU size %d is too small for SPLIT_BUFFER_PDU", maxWireSize)
	}

	encoded := Encode(p)
	if len(encoded) <= maxWireSize {
		return []PDU{p}, nil
	}
	if p.ID == SplitBufferPDU {
		return nil, fmt.Errorf("SPLIT_BUFFER_PDU of %d bytes exceeds maximum %d", len(encoded), maxWireSize)
	}

	chunkSize := maxWireSize - splitHeaderSize
	fragments := make([]PDU, 0, (len(encoded)+chunkSize-1)/chunkSize)
	for offset := 0; offset < len(encoded); {
		end := offset + chunkSize
		if end > len(encoded) {
			end = len(encoded)
		}

		body := make([]byte, 5+end-offset)
		if end == len(encoded) {
			body[0] = 1 // m_lastInSet
		}
		binary.BigEndian.PutUint32(body[1:5], uint32(end-offset))
		copy(body[5:], encoded[offset:end])
		fragments = append(fragments, PDU{ID: SplitBufferPDU, Body: body})
		offset = end
	}
	return fragments, nil
}

func Decode(data []byte) ([]PDU, error) {
	var messages []PDU
	for len(data) > 0 {
		if len(data) < 6 {
			return nil, fmt.Errorf("truncated PDU header: %d bytes", len(data))
		}
		size := int(binary.BigEndian.Uint32(data[2:]))
		if size < 6 || size > len(data) {
			return nil, fmt.Errorf("invalid PDU length %d in %d bytes", size, len(data))
		}
		messages = append(messages, PDU{binary.BigEndian.Uint16(data), data[6:size]})
		data = data[size:]
	}
	return messages, nil
}

// HostGreeting advertises no reserved words, the host version, and buffer size.
// Layout recovered from 0x50c260, 0x503520 and the VersionInfo serializer.
func HostGreeting() PDU {
	body := []byte{0, 0, 1, 5, 2, 2, byte(ClientBufferSize >> 8), byte(ClientBufferSize & 0xff)}
	return PDU{HostOnline, body}
}

// ListRoomsResponse returns an empty room list for the local lobby.
//
// The client class with message ID 0x27 serializes a base uint32, a string,
// then the room count and each room record. Its constructor initializes those
// fields to zero/empty. With no local rooms, the resulting body is 12 zero
// bytes. The base-field meanings are still unknown; the layout comes from the
// client's serializer at 0x509f70 and its base serializer at 0x511e10.
func ListRoomsResponse() PDU {
	return ListRoomsResponseWith(nil)
}

// StartupSettingResponse returns the client's default response for the
// settings it sends immediately after CLIENT_ONLINE. These response objects
// serialize a base uint32 and empty string, followed by a byte setting; the
// ignore-list response instead carries a uint32 limit.
func StartupSettingResponse(requestID uint16) (PDU, bool) {
	var responseID uint16
	bodyLength := 9 // base uint32, empty string, and one setting byte
	switch requestID {
	case SetAcceptAlerts:
		responseID = 0x32
	case SetIgnoreList:
		responseID = 0x34
		bodyLength = 12 // base fields plus the ignored-player limit
	case SetInvincibility:
		responseID = 0x36
	case SetInvisibility:
		responseID = 0x38
	case SetAcceptFlashes:
		responseID = 0x44
	default:
		return PDU{}, false
	}
	return PDU{responseID, make([]byte, bodyLength)}, true
}

// ConfigurationUpdate sends empty deltas for the five static tables. The
// client's serializer writes a sync byte, five counted tables, and a sequence
// byte. Sequence 1 clears the client's cached tables, so this bootstrap uses 0
// and relies on the client's existing cache.
func ConfigurationUpdate(synchronized bool, sequence byte) PDU {
	body := make([]byte, 22)
	if synchronized {
		body[0] = 1
	}
	body[21] = sequence
	return PDU{StaticConfigUpdate, body}
}

// ConfigurationSynchronized is the final step of the startup configuration
// exchange. The client sets its configuration-ready flag after an unsynchronized
// update and replies with STATIC_CONFIG_RESPONSE before this message is sent.
func ConfigurationSynchronized() PDU {
	return ConfigurationUpdate(true, 0)
}

// ParseCreateCharacterRequest reads the call sign and empty inventory list
// from CREATE_CHARACTER. The client serializes the list after the name; its
// element layout is not needed for the empty list used by this bootstrap.
func ParseCreateCharacterRequest(body []byte) (string, error) {
	if len(body) < 8 {
		return "", fmt.Errorf("create-character body is too short: %d bytes", len(body))
	}

	encodedNameLength := binary.BigEndian.Uint32(body[:4])
	if uint64(encodedNameLength) > uint64(len(body)-8) {
		return "", fmt.Errorf("create-character name length %d exceeds body", encodedNameLength)
	}
	nameLength := int(encodedNameLength)
	nameEnd := 4 + nameLength
	name := string(body[4:nameEnd])
	if nameLength < 4 || nameLength > 15 {
		return "", fmt.Errorf("create-character call sign length must be 4–15 bytes, got %d", nameLength)
	}
	if !utf8.ValidString(name) {
		return "", fmt.Errorf("create-character call sign is not valid UTF-8")
	}

	if len(body)-nameEnd != 4 {
		return "", fmt.Errorf("create-character inventory list has %d trailing bytes, want 4", len(body)-nameEnd)
	}
	itemCount := binary.BigEndian.Uint32(body[nameEnd:])
	if itemCount != 0 {
		return "", fmt.Errorf("create-character inventory list has unsupported item count %d", itemCount)
	}
	return name, nil
}

// CreateCharacterCompletion sends the zero-result form of the client's
// CREATE_CHARACTER_FAILED message. The client handler treats result code zero
// as completion and clears the pending form. The local client reached the
// Inner Sphere map shell after this reply, but it remains unverified as the authentic
// success response or as proof of durable character creation.
func CreateCharacterCompletion() PDU {
	// The failed-message base serializer writes a uint32 result and a string.
	// Zero result and empty string therefore produce eight zero bytes.
	return PDU{CreateCharacterReply, make([]byte, 8)}
}

// ParseJoinPlanetRequest reads the selected planet and password from
// JOIN_PLANET. The client serializes the planet ID as a uint32 followed by a
// length-prefixed password string; the observed Solaris request uses ID 121
// and an empty password.
func ParseJoinPlanetRequest(body []byte) (uint32, string, error) {
	if len(body) < 8 {
		return 0, "", fmt.Errorf("join-planet body is too short: %d bytes", len(body))
	}

	planetID := binary.BigEndian.Uint32(body[:4])
	passwordLength := binary.BigEndian.Uint32(body[4:8])
	if uint64(passwordLength) != uint64(len(body)-8) {
		return 0, "", fmt.Errorf("join-planet password length %d does not match %d trailing bytes", passwordLength, len(body)-8)
	}
	password := string(body[8:])
	if !utf8.ValidString(password) {
		return 0, "", fmt.Errorf("join-planet password is not valid UTF-8")
	}
	return planetID, password, nil
}

// ParseExitRoomRequest reads the optional room name from EXIT_ROOM. The live
// client trace contains an empty string after its Solaris JOIN_PLANET request.
func ParseExitRoomRequest(body []byte) (string, error) {
	if len(body) < 4 {
		return "", fmt.Errorf("exit-room body is too short: %d bytes", len(body))
	}

	nameLength := binary.BigEndian.Uint32(body[:4])
	if uint64(nameLength) != uint64(len(body)-4) {
		return "", fmt.Errorf("exit-room name length %d does not match %d trailing bytes", nameLength, len(body)-4)
	}
	name := string(body[4:])
	if !utf8.ValidString(name) {
		return "", fmt.Errorf("exit-room name is not valid UTF-8")
	}
	return name, nil
}

// ExitRoomUpdate serializes the same message class the client sent. This
// acknowledgement is a trial based on the client's bidirectional room
// messages; its success semantics have not yet been confirmed in a live run.
func ExitRoomUpdate(roomName string) PDU {
	return PDU{ExitRoom, appendWireString(nil, roomName)}
}

// JoinPlanetFailure returns the client's typed failure response for an
// invalid or unavailable planet. The client class metadata identifies this
// message as ID 0x92; ID 0x90 is COMBAT_SUMMARY and must not be used here.
func JoinPlanetFailure(result uint32, message string) PDU {
	body := binary.BigEndian.AppendUint32(nil, result)
	body = binary.BigEndian.AppendUint32(body, uint32(len(message)))
	body = append(body, message...)
	return PDU{JoinPlanetFailed, body}
}

// ParsePlanetStatesRequest reads the planet ID requested by the client. The
// observed map screen repeatedly requests planet ID zero to refresh global
// state; the server still needs a real snapshot before it can answer safely.
func ParsePlanetStatesRequest(body []byte) (uint32, error) {
	if len(body) != 4 {
		return 0, fmt.Errorf("planet-states request has %d body bytes, want 4", len(body))
	}
	return binary.BigEndian.Uint32(body), nil
}

// PlanetStateWireEntry is the element shape decoded by the client's
// PLANET_STATE_LIST_UPDATE collection. The first uint32 is used as the
// collection's lookup key; the remaining fields have no recovered gameplay
// names: another uint32, a counted uint16 array, a counted byte array, and one
// trailing byte.
type PlanetStateWireEntry struct {
	Key      uint32
	ValueB   uint32
	Values16 []uint16
	Values8  []byte
	Flag     byte
}

// PlanetStateListUpdate serializes the list element layout recovered from the
// client decoder at BT3025.exe:0x4e58a0 and element codec at 0x517c20/0x517d40.
// The surrounding update then appends its sequence number and shift timer.
func PlanetStateListUpdate(entries []PlanetStateWireEntry, sequence uint32, secondsUntilShift uint16) PDU {
	body := binary.BigEndian.AppendUint32(nil, uint32(len(entries)))
	for _, entry := range entries {
		body = binary.BigEndian.AppendUint32(body, entry.Key)
		body = binary.BigEndian.AppendUint32(body, entry.ValueB)
		body = binary.BigEndian.AppendUint32(body, uint32(len(entry.Values16)))
		for _, value := range entry.Values16 {
			body = binary.BigEndian.AppendUint16(body, value)
		}
		body = binary.BigEndian.AppendUint32(body, uint32(len(entry.Values8)))
		body = append(body, entry.Values8...)
		body = append(body, entry.Flag)
	}
	body = binary.BigEndian.AppendUint32(body, sequence)
	body = binary.BigEndian.AppendUint16(body, secondsUntilShift)
	return PDU{PlanetStateList, body}
}

// SolarisPlanetStateSchemaProbe is a one-entry, structurally valid decoder
// probe. It sets the lookup key to 121; the other fields and authoritative
// Solaris state remain unknown, so callers must opt in.
func SolarisPlanetStateSchemaProbe() PDU {
	return PlanetStateListUpdate([]PlanetStateWireEntry{{Key: 121}}, 0, 0)
}

// FullPlanetStateSchemaProbe tests list cardinality with lookup keys 1 through
// 121. The state fields remain unknown.
func FullPlanetStateSchemaProbe() PDU {
	entries := make([]PlanetStateWireEntry, 121)
	for i := range entries {
		entries[i].Key = uint32(i + 1)
	}
	return PlanetStateListUpdate(entries, 0, 0)
}

// PlanetStateIDMirrorProbe isolates the second scalar: both scalars carry the
// numeric planet ID while the arrays and flag stay at their known-safe zeros.
// Its gameplay meaning is unconfirmed; use only on an isolated listener.
func PlanetStateIDMirrorProbe() PDU {
	entries := make([]PlanetStateWireEntry, 121)
	for i := range entries {
		entries[i].Key = uint32(i + 1)
		entries[i].ValueB = uint32(i + 1)
	}
	return PlanetStateListUpdate(entries, 0, 0)
}

func appendWireString(body []byte, value string) []byte {
	body = binary.BigEndian.AppendUint32(body, uint32(len(value)))
	return append(body, value...)
}

// ParseLobbyRoomsRequest reads the planet whose lobby rooms the client wants.
// The request serializer labels its only field m_planetID at BT3025.exe:0x521f80.
func ParseLobbyRoomsRequest(body []byte) (uint32, error) {
	if len(body) != 4 {
		return 0, fmt.Errorf("lobby-rooms request has %d body bytes, want 4", len(body))
	}
	return binary.BigEndian.Uint32(body), nil
}

// ReadyRoomMapEntry is one entry in READY_ROOM_LIST's ordered auxiliary tree.
// The client compares the key's string content and then KeyDiscriminator; the
// semantic meaning of that discriminator and Value are not yet known.
type ReadyRoomMapEntry struct {
	Key              string
	KeyDiscriminator uint32
	Value            uint32
}

// ReadyRoomListUpdate writes the counted uint32 list at +0x08, followed by
// the ordered tree count at +0x28 and each tree key/value pair. Each key is
// serialized as its discriminator, then a length-prefixed string. The meaning
// of the list elements and tree entries remains unknown.
func ReadyRoomListUpdate(roomInfoList []uint32, mapEntries []ReadyRoomMapEntry) PDU {
	body := binary.BigEndian.AppendUint32(nil, uint32(len(roomInfoList)))
	for _, value := range roomInfoList {
		body = binary.BigEndian.AppendUint32(body, value)
	}
	body = binary.BigEndian.AppendUint32(body, uint32(len(mapEntries)))
	for _, entry := range mapEntries {
		body = binary.BigEndian.AppendUint32(body, entry.KeyDiscriminator)
		body = appendWireString(body, entry.Key)
		body = binary.BigEndian.AppendUint32(body, entry.Value)
	}
	return PDU{ReadyRoomList, body}
}

// EmptyReadyRoomList returns a READY_ROOM_LIST with an empty uint32 list and
// an empty auxiliary tree.
func EmptyReadyRoomList() PDU {
	return ReadyRoomListUpdate(nil, nil)
}
