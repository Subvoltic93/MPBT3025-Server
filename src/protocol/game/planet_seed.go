package protocol

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"os"
)

const (
	planetVCRTableOffset = 0x3848
	planetVCRTableCount  = 121
	planetVCRHash        = "3c17cf8fee9c6bc028e193f455cefda70e00168a68ef92572bc5f27766ad007a"
)

var (
	firstPlanetRecord = []byte("\x00\x00\x00\x01\x00\x00\x00\x0bBuckminster")
	lastPlanetRecord  = []byte("\x00\x00\x00\x79\x00\x00\x00\x07Solaris")
	firstHouseRecord  = []byte("\x00\x00\x00\x00\x00\x00\x00\x04none\x00\x00\x00\x08no house")
)

// PlanetStateListFromVCR builds an experimental 0xa8 update from the planet
// table in the preserved client cache. The VCR record type and the network
// list element type have not been proven to match; live client behavior must
// confirm or reject this seed.
func PlanetStateListFromVCR(path string) (PDU, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return PDU{}, fmt.Errorf("read planet-state seed %q: %w", path, err)
	}
	hash := sha256.Sum256(data)
	if got := hex.EncodeToString(hash[:]); got != planetVCRHash {
		return PDU{}, fmt.Errorf("planet-state seed hash %s does not match the recovered VCR", got)
	}
	if len(data) < planetVCRTableOffset+4 {
		return PDU{}, fmt.Errorf("planet-state seed is too short: %d bytes", len(data))
	}
	if count := binary.BigEndian.Uint32(data[planetVCRTableOffset : planetVCRTableOffset+4]); count != planetVCRTableCount {
		return PDU{}, fmt.Errorf("planet-state seed has %d records, want %d", count, planetVCRTableCount)
	}

	firstRecord := planetVCRTableOffset + 4
	if !bytes.HasPrefix(data[firstRecord:], firstPlanetRecord) {
		return PDU{}, fmt.Errorf("planet-state seed does not start with planet ID 1")
	}
	lastRecord := bytes.Index(data[firstRecord:], lastPlanetRecord)
	if lastRecord < 0 {
		return PDU{}, fmt.Errorf("planet-state seed does not contain Solaris ID 121")
	}
	lastRecord += firstRecord
	houseTable := bytes.Index(data[lastRecord+len(lastPlanetRecord):], firstHouseRecord)
	if houseTable < 0 {
		return PDU{}, fmt.Errorf("planet-state seed has no following HouseInfo table")
	}
	houseTable += lastRecord + len(lastPlanetRecord)
	if houseTable <= firstRecord {
		return PDU{}, fmt.Errorf("planet-state seed has an invalid table boundary")
	}

	// The VCR's counted records begin with the same count field expected by
	// PLANET_STATE_LIST_UPDATE. Append its sequence and shift timer fields.
	body := append([]byte(nil), data[planetVCRTableOffset:houseTable]...)
	body = binary.BigEndian.AppendUint32(body, 0) // sequence number
	body = binary.BigEndian.AppendUint16(body, 0) // seconds until shift
	return PDU{PlanetStateList, body}, nil
}

// SolarisPlanetStateListFromVCR builds a small 0xa8 trial containing only the
// Solaris cache record. It keeps the packet below the client's advertised
// buffer size so we can test the record decoder without split-buffer framing.
// The cache record is still only a candidate network element, not a confirmed
// live planet-state representation.
func SolarisPlanetStateListFromVCR(path string) (PDU, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return PDU{}, fmt.Errorf("read planet-state seed %q: %w", path, err)
	}
	hash := sha256.Sum256(data)
	if got := hex.EncodeToString(hash[:]); got != planetVCRHash {
		return PDU{}, fmt.Errorf("planet-state seed hash %s does not match the recovered VCR", got)
	}

	solaris := bytes.Index(data, lastPlanetRecord)
	if solaris < planetVCRTableOffset+4 {
		return PDU{}, fmt.Errorf("planet-state seed does not contain Solaris ID 121")
	}
	houseTable := bytes.Index(data[solaris+len(lastPlanetRecord):], firstHouseRecord)
	if houseTable < 0 {
		return PDU{}, fmt.Errorf("planet-state seed has no following HouseInfo table")
	}
	houseTable += solaris + len(lastPlanetRecord)
	if houseTable-solaris != 186 {
		return PDU{}, fmt.Errorf("Solaris cache record is %d bytes, want the recovered 186-byte record", houseTable-solaris)
	}

	body := binary.BigEndian.AppendUint32(nil, 1)
	body = append(body, data[solaris:houseTable]...)
	body = binary.BigEndian.AppendUint32(body, 0) // sequence number
	body = binary.BigEndian.AppendUint16(body, 0) // seconds until shift
	return PDU{PlanetStateList, body}, nil
}
