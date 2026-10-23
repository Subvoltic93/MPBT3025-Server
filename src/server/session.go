package server

import (
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"mpbt3025/src/protocol/game"
	"mpbt3025/src/state"
)

type liveSession struct {
	sendGamePDU func(protocol.PDU) error
	knownRooms  map[uint32]bool
}

// sessionHub fans out live occupant arrival/departure events between
// currently connected clients. Persistent pilot locations alone are not
// sufficient for planet occupancy: pilots from earlier disconnected runs
// must not appear as live players.
type sessionHub struct {
	mu                           sync.Mutex
	sessions                     map[*liveSession]planetPresence
	world                        *state.Store
	planetRoomType               int // -1 disables the recovered room-update live trial
	experimentalCombatInit       bool
	experimentalCombatArmorProbe bool
	experimentalCombatDamage     bool
	combatDamageRooms            map[int]*combatDamageRoom
	combatArmorProbed            map[int]map[byte]bool
	combatPreparing              map[int]map[string]bool
	combatEntities               map[int]map[string]byte
	// Client reports support diagnostics and the opt-in damage plausibility policy.
	combatPositionReports map[int]map[byte]protocol.CombatPosition
	combatPositionTimes   map[int]map[byte]time.Time
	combatActive          map[int]bool
}

type planetPresence struct {
	callSign string
	location state.Location
	houseID  state.HouseID
}

// Solaris staging rooms belong to a city. Their roster includes that city's
// seated pilots, but must not include pilots from another city's staging area.
func samePresenceArea(a, b state.Location) bool {
	return a.PlanetID == b.PlanetID && (a.PlanetID != state.SolarisPlanetID || a.CityID == b.CityID)
}

func solarisLobbyRoomID(cityID int) protocol.RoomID {
	return protocol.RoomID{Number: uint32(121000 + cityID), Host: "local"}
}

func solarisMissionRooms(world *state.Store, pilot state.Pilot) ([]protocol.RoomInfo, error) {
	missions, err := world.RoomsForSolarisCity(pilot.CallSign)
	if err != nil {
		return nil, err
	}
	parent := solarisLobbyRoomID(pilot.Location.CityID)
	rooms := make([]protocol.RoomInfo, 0, len(missions))
	for _, mission := range missions {
		open := [2]byte{state.LanceSeatCount, state.LanceSeatCount}
		var occupied uint32
		for side, seats := range mission.Seats {
			for _, seat := range seats {
				if seat.CallSign != "" || seat.AI {
					open[side]--
					occupied++
				}
			}
		}
		roomType := byte(4)
		if mission.CadetOnly {
			roomType = 5
		}
		rooms = append(rooms, protocol.RoomInfo{
			ID:   protocol.RoomID{Number: uint32(mission.ID), Host: "local"},
			Name: mission.Name, Stage: parent, Parent: parent, Type: roomType,
			Occupancy: occupied, TotalOccupancy: uint16(occupied), Capacity: 8,
			Description: mission.Mission, PlanetID: state.SolarisPlanetID,
			Sanctioned: mission.CadetOnly, Visible: true,
			AttackerSeats: state.LanceSeatCount, DefenderSeats: state.LanceSeatCount,
			// Solaris lists defender seats as Alpha and attacker seats as Beta
			// (native row renderer 0x45c6f4/0x45c781).
			OpenAttackerSeats: open[1], OpenDefenderSeats: open[0],
		})
	}
	return rooms, nil
}

func newSessionHub(worlds ...*state.Store) *sessionHub {
	h := &sessionHub{sessions: make(map[*liveSession]planetPresence), planetRoomType: -1}
	if len(worlds) > 0 {
		h.world = worlds[0]
	}
	return h
}

func (h *sessionHub) remove(session *liveSession) {
	if h == nil || session == nil {
		return
	}
	h.mu.Lock()
	presence, joined := h.sessions[session]
	delete(h.sessions, session)
	if joined && presence.location.RoomID != 0 {
		delete(h.combatPreparing, presence.location.RoomID)
		delete(h.combatEntities, presence.location.RoomID)
		delete(h.combatPositionReports, presence.location.RoomID)
		delete(h.combatPositionTimes, presence.location.RoomID)
		delete(h.combatDamageRooms, presence.location.RoomID)
		delete(h.combatActive, presence.location.RoomID)
		delete(h.combatArmorProbed, presence.location.RoomID)
	}
	others := make([]*liveSession, 0, len(h.sessions))
	if joined {
		for other, otherPresence := range h.sessions {
			if samePresenceArea(otherPresence.location, presence.location) && (otherPresence.location.RoomID == 0 || otherPresence.location.RoomID == presence.location.RoomID) {
				others = append(others, other)
			}
		}
	}
	stillConnected := false
	for _, p := range h.sessions {
		if strings.EqualFold(p.callSign, presence.callSign) {
			stillConnected = true
		}
	}
	h.mu.Unlock()
	if stillConnected {
		return
	}
	if joined && h.world != nil {
		if err := h.world.LeaveRoom(presence.callSign); err != nil {
			log.Printf("Release disconnected pilot %q: %v", presence.callSign, err)
		}
	}
	for _, other := range others {
		if err := other.sendGamePDU(protocol.OccupantDepartedFor(presence.callSign)); err != nil {
			log.Printf("Could not publish departing occupant %q to another planet client: %v", presence.callSign, err)
		}
	}
	if joined {
		h.refreshMission(presence.location.RoomID)
		h.publishRoomLists()
	}
}

// publishRoomLists updates only clients viewing a Solaris staging area. Each
// client remembers its announced rooms so new empty slots use Added, changed
// rooms use Update, and missions that launched disappear via Removed.
func (h *sessionHub) publishRoomLists() {
	if h == nil || h.world == nil || h.planetRoomType < 0 {
		return
	}
	h.mu.Lock()
	targets := make(map[*liveSession]planetPresence)
	for session, p := range h.sessions {
		if p.location.PlanetID == state.SolarisPlanetID && p.location.RoomID == 0 {
			targets[session] = p
		}
	}
	h.mu.Unlock()
	for session, p := range targets {
		pilot, ok := h.world.Pilot(p.callSign)
		if !ok {
			continue
		}
		rooms, err := solarisMissionRooms(h.world, pilot)
		if err != nil {
			log.Printf("Refresh room list for %q: %v", p.callSign, err)
			continue
		}
		updates := make([]protocol.PDU, 0, len(rooms))
		h.mu.Lock()
		if session.knownRooms == nil {
			session.knownRooms = make(map[uint32]bool)
		}
		current := make(map[uint32]bool, len(rooms))
		for _, room := range rooms {
			current[room.ID.Number] = true
			seats := missionSeatLites(h.world, room.ID.Number)
			if session.knownRooms[room.ID.Number] {
				updates = append(updates, protocol.ReadyRoomUpdate(room, seats))
			} else {
				updates = append(updates, protocol.ReadyRoomAdded(room, seats))
			}
		}
		for id := range session.knownRooms {
			if !current[id] {
				updates = append(updates, protocol.ReadyRoomRemoved(protocol.RoomID{Number: id, Host: "local"}))
			}
		}
		session.knownRooms = current
		h.mu.Unlock()
		for _, update := range updates {
			if err := session.sendGamePDU(update); err != nil {
				log.Printf("Publish room list for %q: %v", p.callSign, err)
				break
			}
		}
	}
}

func (h *sessionHub) refreshMission(roomID int) {
	if h == nil || h.world == nil || roomID == 0 {
		return
	}
	h.mu.Lock()
	targets := make([]*liveSession, 0)
	for session, p := range h.sessions {
		if p.location.RoomID == roomID {
			targets = append(targets, session)
		}
	}
	h.mu.Unlock()
	combat := missionCombat(h.world, protocol.RoomInfo{ID: protocol.RoomID{Number: uint32(roomID), Host: "local"}})
	timer := missionCountdown(h.world, roomID, time.Now())
	for _, session := range targets {
		if err := session.sendGamePDU(combat); err != nil {
			log.Printf("Refresh mission %d: %v", roomID, err)
			continue
		}
		if err := session.sendGamePDU(timer); err != nil {
			log.Printf("Refresh mission timer %d: %v", roomID, err)
		}
	}
}

func (h *sessionHub) publishMissionDeparture(session *liveSession, previous planetPresence, pilot state.Pilot) {
	h.mu.Lock()
	delete(h.combatPreparing, previous.location.RoomID)
	delete(h.combatEntities, previous.location.RoomID)
	delete(h.combatPositionReports, previous.location.RoomID)
	delete(h.combatPositionTimes, previous.location.RoomID)
	delete(h.combatDamageRooms, previous.location.RoomID)
	delete(h.combatActive, previous.location.RoomID)
	delete(h.combatArmorProbed, previous.location.RoomID)
	targets := make([]*liveSession, 0)
	for other, p := range h.sessions {
		if other != session && previous.location.RoomID != 0 && p.location.RoomID == previous.location.RoomID {
			targets = append(targets, other)
		}
	}
	h.mu.Unlock()
	for _, other := range targets {
		if err := other.sendGamePDU(protocol.OccupantDepartedFor(previous.callSign)); err != nil {
			log.Printf("Mission departure: %v", err)
		}
	}
	// Synchronize the leaving client's combat context before changing rooms.
	// Root RoomUpdate does not clear CombatManager's local seat; without this
	// snapshot the native client remains seated in its previous mission.
	if h.world != nil && previous.location.RoomID != 0 {
		info := protocol.RoomInfo{ID: protocol.RoomID{Number: uint32(previous.location.RoomID), Host: "local"}}
		if err := session.sendGamePDU(missionCombat(h.world, info)); err != nil {
			log.Printf("Clear departed mission seat: %v", err)
		}
	}
	h.publishPlanetJoin(session, pilot)
	h.refreshMission(previous.location.RoomID)
	h.publishRoomLists()
}

func (h *sessionHub) publishPlanetJoin(session *liveSession, pilot state.Pilot) {
	if session == nil || session.sendGamePDU == nil {
		return
	}
	if h == nil {
		if err := session.sendGamePDU(protocol.OccupantArrivedFor(pilot.CallSign)); err != nil {
			log.Printf("Could not update local occupant %q: %v", pilot.CallSign, err)
		}
		return
	}

	h.mu.Lock()
	existing := make([]planetPresence, 0, len(h.sessions))
	existingSessions := make([]*liveSession, 0, len(h.sessions))
	for other, presence := range h.sessions {
		if other != session && samePresenceArea(presence.location, pilot.Location) {
			existing = append(existing, presence)
			existingSessions = append(existingSessions, other)
		}
	}
	if pilot.Location.RoomID == 0 {
		session.knownRooms = nil
	}
	h.sessions[session] = planetPresence{
		callSign: pilot.CallSign,
		location: pilot.Location,
		houseID:  pilot.House,
	}
	h.mu.Unlock()

	var rosterRoom *protocol.RoomInfo
	// Occupants are keyed by their string PlayerID. PLAYER_UPDATE (0x3c)
	// replaces the receiving client's own pilot and must never carry a peer.
	if h.planetRoomType >= 0 && pilot.Location.PlanetID == state.SolarisPlanetID {
		roomID := solarisLobbyRoomID(pilot.Location.CityID)
		room := protocol.RoomInfo{
			ID: roomID, Name: "Solaris", Stage: roomID,
			Occupancy: uint32(len(existing) + 1), Capacity: 100,
			Type: byte(h.planetRoomType), TotalOccupancy: uint16(len(existing) + 1),
			AllowPlayerRooms: true, MaxRank: 255, PlanetID: state.SolarisPlanetID, Visible: true,
		}
		rosterRoom = &room
		if err := session.sendGamePDU(protocol.RoomUpdate(1, true, room)); err != nil {
			log.Printf("Could not publish experimental room update for %q: %v", pilot.CallSign, err)
		}
	}
	if err := session.sendGamePDU(protocol.OccupantUpdateFor(pilot.CallSign)); err != nil {
		log.Printf("Could not update local occupant %q for joining pilot: %v", pilot.CallSign, err)
	}
	// Existing clients receive the new peer's occupant update. The joining
	// client has already received its own 0x94 before those peer entries.
	for _, presence := range existing {
		if err := session.sendGamePDU(protocol.OccupantUpdateFor(presence.callSign)); err != nil {
			log.Printf("Could not update existing occupant %q for joining pilot %q: %v", presence.callSign, pilot.CallSign, err)
		}

		if h.planetRoomType >= 0 && pilot.Location.PlanetID == state.SolarisPlanetID {
			if err := session.sendGamePDU(protocol.OccupantArrivedFor(presence.callSign)); err != nil {
				log.Printf("Rebuild occupant roster: %v", err)
			}
		}
	}

	for _, other := range existingSessions {
		h.mu.Lock()
		target := h.sessions[other]
		h.mu.Unlock()
		if target.location.RoomID != 0 && target.location.RoomID != pilot.Location.RoomID {
			continue
		}
		if err := other.sendGamePDU(protocol.OccupantUpdateFor(pilot.CallSign)); err != nil {
			log.Printf("Could not update arriving occupant %q for another planet client: %v", pilot.CallSign, err)
		}
		if h.planetRoomType >= 0 && pilot.Location.PlanetID == state.SolarisPlanetID {
			if err := other.sendGamePDU(protocol.OccupantArrivedFor(pilot.CallSign)); err != nil {
				log.Printf("Announce occupant arrival: %v", err)
			}
		}
	}
	if err := session.sendGamePDU(protocol.OccupantArrivedFor(pilot.CallSign)); err != nil {
		log.Printf("Could not announce local occupant %q for joining pilot %q: %v", pilot.CallSign, pilot.CallSign, err)
	}

	// Finish with the complete membership snapshot after individual cache updates.
	if rosterRoom != nil {
		roster := []string{pilot.CallSign}
		for _, p := range existing {
			roster = append(roster, p.callSign)
		}
		if err := session.sendGamePDU(protocol.UpdateOccupants(*rosterRoom, roster)); err != nil {
			log.Printf("Rebuild planet roster: %v", err)
		}
	}
}

// publishMissionJoin changes the active room and rebuilds that room's roster.
func (h *sessionHub) publishMissionJoin(session *liveSession, pilot state.Pilot, room protocol.RoomInfo, combat protocol.PDU, seatUpdates []protocol.CombatSeat) {
	h.mu.Lock()
	previous := h.sessions[session]
	h.sessions[session] = planetPresence{callSign: pilot.CallSign, location: pilot.Location, houseID: pilot.House}
	peers := make(map[*liveSession]planetPresence)
	for other, presence := range h.sessions {
		if other != session && samePresenceArea(presence.location, pilot.Location) {
			peers[other] = presence
		}
	}
	h.mu.Unlock()
	send := func(target *liveSession, p protocol.PDU) {
		if err := target.sendGamePDU(p); err != nil {
			log.Printf("Mission update for %q: %v", pilot.CallSign, err)
		}
	}
	send(session, protocol.PlayerUpdateFor(pilot.CallSign))
	send(session, protocol.RoomUpdate(1, true, room))
	send(session, combat)
	for _, seatUpdate := range seatUpdates {
		send(session, protocol.SeatOptionsUpdate(seatUpdate, 0))
	}
	send(session, protocol.OccupantUpdateFor(pilot.CallSign))
	for other, presence := range peers {
		if presence.location.RoomID == pilot.Location.RoomID {
			send(session, protocol.OccupantUpdateFor(presence.callSign))
			send(session, protocol.OccupantArrivedFor(presence.callSign))
			send(other, protocol.OccupantUpdateFor(pilot.CallSign))
			send(other, protocol.OccupantArrivedFor(pilot.CallSign))
			send(other, combat)
			for _, seatUpdate := range seatUpdates {
				send(other, protocol.SeatOptionsUpdate(seatUpdate, 0))
			}
		}
	}
	send(session, protocol.OccupantArrivedFor(pilot.CallSign))
	roster := []string{pilot.CallSign}
	for _, p := range peers {
		if p.location.RoomID == pilot.Location.RoomID {
			roster = append(roster, p.callSign)
		}
	}
	send(session, protocol.UpdateOccupants(room, roster))
	if previous.location.RoomID != 0 && previous.location.RoomID != pilot.Location.RoomID {
		for other, p := range peers {
			if p.location.RoomID == previous.location.RoomID {
				send(other, protocol.OccupantDepartedFor(pilot.CallSign))
			}
		}
		h.refreshMission(previous.location.RoomID)
	}
	h.publishRoomLists()
}

func (h *sessionHub) publishMissionSeatUpdate(pilot state.Pilot, seat protocol.CombatSeat) {
	if h == nil {
		return
	}
	h.mu.Lock()
	recipients := make([]*liveSession, 0, len(h.sessions))
	for session, presence := range h.sessions {
		if presence.location.PlanetID == pilot.Location.PlanetID && presence.location.RoomID == pilot.Location.RoomID {
			recipients = append(recipients, session)
		}
	}
	h.mu.Unlock()
	update := protocol.SeatOptionsUpdate(seat, 0)
	for _, session := range recipients {
		if err := session.sendGamePDU(update); err != nil {
			log.Printf("Mission seat update for %q: %v", pilot.CallSign, err)
		}
	}
}

func (h *sessionHub) publishMissionCombatUpdate(pilot state.Pilot, combat protocol.PDU) {
	if h == nil {
		return
	}
	h.mu.Lock()
	recipients := make([]*liveSession, 0, len(h.sessions))
	for session, presence := range h.sessions {
		if presence.location.PlanetID == pilot.Location.PlanetID && presence.location.RoomID == pilot.Location.RoomID {
			recipients = append(recipients, session)
		}
	}
	h.mu.Unlock()
	for _, session := range recipients {
		if err := session.sendGamePDU(combat); err != nil {
			log.Printf("Mission combat update for %q: %v", pilot.CallSign, err)
		}
	}
}

func (h *sessionHub) publishMissionLaunch(roomID int, enter protocol.PDU) {
	if h == nil {
		return
	}
	h.mu.Lock()
	recipients := make([]*liveSession, 0, len(h.sessions))
	for session, presence := range h.sessions {
		if presence.location.RoomID == roomID {
			recipients = append(recipients, session)
		}
	}
	h.mu.Unlock()
	for _, session := range recipients {
		if err := session.sendGamePDU(enter); err != nil {
			log.Printf("Mission launch for room %d: %v", roomID, err)
		}
	}
}

// nativeCombatSpawn places the experimental lances on opposite sides of the
// origin, with 30 metres between seats and 200 metres between the first pair.
// FillMech (0x6904207e..0x690420fd) scales coordinates by .01 and faces each
// Mech toward the origin. Side-by-side spawns make a controlled shot difficult.
func nativeCombatSpawn(side, slot int) (int32, int32) {
	x, y := int32(slot)*3000, int32(10000)
	if side == 1 {
		x, y = -x, -y
	}
	return x, y
}

func (h *sessionHub) launchMission(roomID int) {
	h.publishMissionLaunch(roomID, missionCurrentCombat(h.world, roomID))
	h.publishMissionLaunch(roomID, protocol.EnterCombat())
	if !h.experimentalCombatInit {
		return
	}
	room := h.world.Snapshot().Rooms[roomID]
	players := []protocol.CombatPilot{}
	ids := map[string]byte{}
	for side, lance := range room.Seats {
		for slot, seat := range lance {
			if seat.CallSign == "" {
				continue
			}
			if seat.MechID != 8 {
				log.Printf("Native combat setup for room %d requires starter Mech 8; %q selected %d", roomID, seat.CallSign, seat.MechID)
				return
			}
			id := byte(len(players))
			ids[seat.CallSign] = id
			x, y := nativeCombatSpawn(side, slot)
			players = append(players, protocol.CombatPilot{ID: id, Team: byte(side), PlayerID: seat.CallSign, CallSign: seat.CallSign, X: x, Y: y})
		}
	}
	h.mu.Lock()
	recipients := map[*liveSession]string{}
	if h.combatPreparing == nil {
		h.combatPreparing = map[int]map[string]bool{}
	}
	pending := map[string]bool{}
	for session, presence := range h.sessions {
		if presence.location.RoomID == roomID {
			recipients[session] = presence.callSign
			pending[presence.callSign] = false
		}
	}
	h.combatPreparing[roomID] = pending
	if h.combatEntities == nil {
		h.combatEntities = map[int]map[string]byte{}
	}
	h.combatEntities[roomID] = ids
	if h.combatPositionReports == nil {
		h.combatPositionReports = map[int]map[byte]protocol.CombatPosition{}
	}
	delete(h.combatPositionReports, roomID)
	delete(h.combatPositionTimes, roomID)
	if h.combatDamageRooms == nil {
		h.combatDamageRooms = map[int]*combatDamageRoom{}
	}
	delete(h.combatDamageRooms, roomID)
	if h.experimentalCombatDamage {
		h.combatDamageRooms[roomID] = newCombatDamageRoom(ids)
	}
	if h.combatActive == nil {
		h.combatActive = map[int]bool{}
	}
	delete(h.combatActive, roomID)
	delete(h.combatArmorProbed, roomID)
	h.mu.Unlock()
	for session, callSign := range recipients {
		localID, ok := ids[callSign]
		if !ok {
			continue
		}
		setup, err := protocol.CombatBootstrap(localID, players)
		if err == nil {
			err = session.sendGamePDU(setup)
		}
		if err != nil {
			log.Printf("Native combat setup for %q: %v", callSign, err)
		} else {
			log.Printf("Sent native combat setup to %q: entity %d, %d COM-4Ds, %d bytes", callSign, localID, len(players), len(setup.Body))
		}
	}
}

// Native outbound position streaming begins after PreCombatInit initializes
// the HUD and advances state 2 to 3. Waiting for every seated client avoids
// calling ReadyForCombat while CHUDBogie still has a null K2D node.
func (h *sessionHub) noteCombatPrepared(session *liveSession, body []byte) {
	if h == nil || !h.experimentalCombatInit {
		return
	}
	commands, err := protocol.ParseClientCombatData(body)
	if err != nil {
		log.Printf("Ignoring invalid native combat input: %v", err)
		return
	}
	if _, ok := protocol.ClientCombatPosition(commands); !ok {
		return
	}
	h.mu.Lock()
	presence := h.sessions[session]
	roomID := presence.location.RoomID
	pending := h.combatPreparing[roomID]
	_, expected := pending[presence.callSign]
	if !expected {
		h.mu.Unlock()
		return
	}
	pending[presence.callSign] = true
	for _, prepared := range pending {
		if !prepared {
			h.mu.Unlock()
			return
		}
	}
	delete(h.combatPreparing, roomID)
	h.mu.Unlock()
	log.Printf("All %d native clients in mission %d prepared; releasing combat", len(pending), roomID)
	h.publishMissionLaunch(roomID, protocol.CombatData(protocol.CombatReadyCommand()))
	h.mu.Lock()
	if _, stillLaunched := h.combatEntities[roomID]; stillLaunched {
		h.combatActive[roomID] = true
	}
	h.mu.Unlock()
}

func (h *sessionHub) relayCombatCommands(session *liveSession, body []byte) {
	if h == nil || !h.experimentalCombatInit {
		return
	}
	commands, err := protocol.ParseClientCombatData(body)
	if err != nil {
		return
	}
	batch, err := protocol.ParseClientCombatCommands(commands)
	if err != nil {
		return
	}
	h.mu.Lock()
	presence, joined := h.sessions[session]
	roomID := presence.location.RoomID
	entityID, expected := h.combatEntities[roomID][presence.callSign]
	if !joined || !h.combatActive[roomID] || !expected {
		h.mu.Unlock()
		return
	}
	if batch.Position != nil {
		if report, parseErr := protocol.ParseClientCombatPositionCommand(batch.Position); parseErr == nil {
			if h.combatPositionReports == nil {
				h.combatPositionReports = map[int]map[byte]protocol.CombatPosition{}
			}
			if h.combatPositionReports[roomID] == nil {
				h.combatPositionReports[roomID] = map[byte]protocol.CombatPosition{}
			}
			// Coordinates remain client supplied. The opt-in damage mode uses
			// them for a coarse plausibility policy, not authoritative collision.
			h.combatPositionReports[roomID][entityID] = report
			if h.combatPositionTimes == nil {
				h.combatPositionTimes = map[int]map[byte]time.Time{}
			}
			if h.combatPositionTimes[roomID] == nil {
				h.combatPositionTimes[roomID] = map[byte]time.Time{}
			}
			h.combatPositionTimes[roomID][entityID] = time.Now()
		}
	}
	recipients := make([]*liveSession, 0)
	for other, peer := range h.sessions {
		if other != session && peer.location.RoomID == roomID {
			if _, member := h.combatEntities[roomID][peer.callSign]; member {
				recipients = append(recipients, other)
			}
		}
	}
	type relayFire struct {
		pdu  protocol.PDU
		echo bool
	}
	fires := make([]relayFire, 0, len(batch.Fires))
	for _, clientFire := range batch.Fires {
		fire, err := combatFireForRoom(entityID, h.combatEntities[roomID], clientFire)
		if err != nil {
			log.Printf("Ignoring native combat fire in mission %d from %q: %v", roomID, presence.callSign, err)
			continue
		}
		selection, _ := protocol.ParseClientCombatFire(clientFire)
		log.Printf("Relay native combat fire in mission %d from %q: weapon slot %d, target entity %d, hit part %d", roomID, presence.callSign, selection.WeaponSlot, selection.TargetEntity, selection.HitPart)
		echo := false
		if damage, ok := h.damageForShot(roomID, entityID, selection, clientFire, time.Now()); ok {
			fire, echo = damage, true
		} else if probe, ok := h.armorProbeForShot(roomID, entityID, selection, clientFire); ok {
			fire, echo = probe, true
		}
		fires = append(fires, relayFire{pdu: fire, echo: echo})
	}
	h.mu.Unlock()
	if batch.Position != nil {
		command, err := protocol.CombatPeerPosition(entityID, batch.Position)
		if err != nil {
			return
		}
		position := protocol.CombatData(command)
		for _, other := range recipients {
			if err := other.sendGamePDU(position); err != nil {
				log.Printf("Native combat position relay in mission %d: %v", roomID, err)
			}
		}
	}
	for _, fire := range fires {
		if fire.echo {
			if err := session.sendGamePDU(fire.pdu); err != nil {
				log.Printf("Native armor probe echo in mission %d: %v", roomID, err)
			}
		}
		for _, other := range recipients {
			if err := other.sendGamePDU(fire.pdu); err != nil {
				log.Printf("Native combat fire relay in mission %d: %v", roomID, err)
			}
		}
	}
}

func combatFireForRoom(shooter byte, roster map[string]byte, command []byte) (protocol.PDU, error) {
	selection, err := protocol.ParseClientCombatFire(command)
	if err != nil {
		return protocol.PDU{}, err
	}
	// launchMission only initializes the stock COM-4D. An arbitrary slot can
	// make the native fire visitor dereference a nonexistent weapon.
	if selection.WeaponSlot >= protocol.CombatCommandoWeaponCount {
		return protocol.PDU{}, fmt.Errorf("weapon slot %d is outside the COM-4D loadout", selection.WeaponSlot)
	}
	if selection.TargetEntity >= 0 {
		// SendShotCommand copies HitReturn's Mech part plus one. The native
		// armor resolver indexes exactly eleven armor regions with this value.
		if selection.HitPart < 0 || selection.HitPart >= protocol.CombatArmorRegionCount {
			return protocol.PDU{}, fmt.Errorf("Mech hit part %d is outside the native armor regions", selection.HitPart)
		}
		found := false
		for _, entity := range roster {
			if int(entity) == selection.TargetEntity {
				found = true
				break
			}
		}
		if !found {
			return protocol.PDU{}, fmt.Errorf("target entity %d is outside the launched room", selection.TargetEntity)
		}
	}
	peer, err := protocol.CombatPeerFire(shooter, command)
	if err != nil {
		return protocol.PDU{}, err
	}
	return protocol.CombatData(peer), nil
}

func startMissionScheduler(world *state.Store, hub *sessionHub) {
	go func() {
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		for now := range ticker.C {
			started, err := world.StartExpiredBattles(now)
			if err != nil {
				log.Printf("Advance expired mission countdowns: %v", err)
				continue
			}
			for _, room := range started {
				log.Printf("Mission %d countdown expired; entering combat", room.ID)
				hub.launchMission(room.ID)
			}
			if len(started) > 0 {
				hub.publishRoomLists()
			}
		}
	}()
}

func missionCombat(world *state.Store, info protocol.RoomInfo) protocol.PDU {
	room := world.Snapshot().Rooms[int(info.ID.Number)]
	seats := missionCombatSeats(room)
	return protocol.CombatConfigurationUpdate(info.ID, state.SolarisPlanetID, 0, 0, seats, 0)
}

func missionCurrentCombat(world *state.Store, roomID int) protocol.PDU {
	room := world.Snapshot().Rooms[roomID]
	roomIDWire := protocol.RoomID{Number: uint32(roomID), Host: "local"}
	return protocol.CurrentCombat(roomIDWire, state.SolarisPlanetID, 0, 0, missionCombatSeats(room))
}

func missionCombatSeats(room state.MissionRoom) []protocol.CombatSeat {
	seats := []protocol.CombatSeat{}
	for side, lance := range room.Seats {
		for index, seat := range lance {
			seats = append(seats, protocol.CombatSeat{SeatLite: protocol.SeatLite{Number: missionSeatNumber(side, index), Team: missionTeam(side), CallSign: seat.CallSign}, OccupantID: seat.CallSign, MechID: seat.MechID, State: missionCombatSeatState(seat)})
		}
	}
	return seats
}

func missionCountdown(world *state.Store, roomID int, now time.Time) protocol.PDU {
	room, ok := world.Snapshot().Rooms[roomID]
	if !ok || room.Status != state.RoomCountdown || !room.CountdownEnds.After(now) {
		return protocol.CountdownTimer(0, protocol.CountdownStopped)
	}
	// Round up so a new recipient never displays zero before the deadline.
	seconds := uint32((room.CountdownEnds.Sub(now) + time.Second - 1) / time.Second)
	return protocol.CountdownTimer(seconds, protocol.CountdownAutoLaunch)
}

func missionCombatSeatState(seat state.Seat) byte {
	if seat.Ready {
		return protocol.CombatSeatStateReady
	}
	return protocol.CombatSeatStateNotReady
}

// CombatInfo's native seat map comparator keys only on CombatSeatLite.Number,
// so the two lances need distinct wire seat numbers even though each lance
// has its own local seat indexes. Combat manager 0x4d8a70 treats number 0
// as unseated, so native seat numbers are one-based.
func missionSeatNumber(side, index int) byte { return byte(side*4 + index + 1) }

// Native Mission Room heading selection at 0x460da9 recognizes 1=Alpha,
// 2=Beta; zero has no team heading.
func missionTeam(side int) byte { return byte(side + 1) }

func missionSeatUpdates(world *state.Store, roomID int) []protocol.CombatSeat {
	room := world.Snapshot().Rooms[roomID]
	seats := []protocol.CombatSeat{}
	for side, lance := range room.Seats {
		for index, seat := range lance {
			if seat.CallSign == "" {
				continue
			}
			seats = append(seats, protocol.CombatSeat{
				SeatLite:   protocol.SeatLite{Number: missionSeatNumber(side, index), Team: missionTeam(side), CallSign: seat.CallSign},
				OccupantID: seat.CallSign,
				MechID:     seat.MechID,
				State:      missionCombatSeatState(seat),
			})
		}
	}
	return seats
}

func missionSeatLites(world *state.Store, roomID uint32) []protocol.SeatLite {
	snapshot := world.Snapshot()
	room := snapshot.Rooms[int(roomID)]
	seats := []protocol.SeatLite{}
	for side, lance := range room.Seats {
		for index, seat := range lance {
			if seat.CallSign != "" {
				seats = append(seats, protocol.SeatLite{Number: missionSeatNumber(side, index), Team: missionTeam(side), CallSign: seat.CallSign})
			}
		}
	}
	return seats
}

func (h *sessionHub) publishPlanetChat(sender *liveSession, message protocol.PDU) {
	if h == nil || sender == nil {
		return
	}
	h.mu.Lock()
	presence, joined := h.sessions[sender]
	if !joined || message.ID != protocol.ChatMessageID || len(message.Body) == 0 {
		h.mu.Unlock()
		return
	}
	var err error
	message, err = protocol.ChatMessageForDisplay(message, presence.callSign)
	if err != nil {
		h.mu.Unlock()
		log.Printf("Ignoring malformed chat from %q: %v", presence.callSign, err)
		return
	}
	channel := message.Body[len(message.Body)-1]
	recipients := make([]*liveSession, 0, len(h.sessions))
	for session, otherPresence := range h.sessions {
		if session == sender {
			recipients = append(recipients, session)
			continue
		}
		if !sameChatLocation(presence.location, otherPresence.location) {
			continue
		}
		switch channel {
		case protocol.ChatChannelAll:
			recipients = append(recipients, session)
		case protocol.ChatChannelTeam:
			// Team chat is restricted to seated Alpha/Beta pilots in the same
			// room. A room observer has no valid lance side and only receives All.
			if presence.location.RoomID != 0 && (presence.location.Side == 0 || presence.location.Side == 1) &&
				otherPresence.location.RoomID == presence.location.RoomID && otherPresence.location.Side == presence.location.Side {
				recipients = append(recipients, session)
			}
		case protocol.ChatChannelHouse:
			if presence.houseID != "" && otherPresence.houseID == presence.houseID {
				recipients = append(recipients, session)
			}
		default:
			// Do not fan out channel values without verified routing semantics.
		}
	}
	h.mu.Unlock()

	for _, recipient := range recipients {
		if err := recipient.sendGamePDU(message); err != nil {
			log.Printf("Could not relay chat from %q to a client on planet %d: %v", presence.callSign, presence.location.PlanetID, err)
		}
	}
}

func sameChatLocation(left, right state.Location) bool {
	if left.PlanetID != right.PlanetID {
		return false
	}
	if left.RoomID != 0 || right.RoomID != 0 {
		return left.RoomID != 0 && left.RoomID == right.RoomID
	}
	if left.SectorID != 0 || right.SectorID != 0 {
		return left.SectorID != 0 && left.SectorID == right.SectorID
	}
	return left.CityID == right.CityID
}
