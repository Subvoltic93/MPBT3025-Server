package server

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"log"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"mpbt3025/src/protocol/aries"
	"mpbt3025/src/protocol/game"
	"mpbt3025/src/state"
)

func serveConnections(listener net.Listener, traceDir string, planetStateReply *protocol.PDU, world *state.Store, redirectAddress string, hub *sessionHub, emptySolarisRoomListAfterJoin bool) {
	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Printf("Secondary listener stopped: %v", err)
			return
		}
		go func() {
			defer conn.Close()
			if err := serve(conn, traceDir, planetStateReply, world, redirectAddress, hub, emptySolarisRoomListAfterJoin); err != nil {
				log.Printf("%s: %v", conn.RemoteAddr(), err)
			}
		}()
	}
}

func serve(conn net.Conn, traceDir string, planetStateReply *protocol.PDU, world *state.Store, redirectAddress string, hub *sessionHub, emptySolarisRoomListAfterJoin bool) error {
	trace, err := os.OpenFile(filepath.Join(traceDir, fmt.Sprintf("session-%d.log", time.Now().UnixNano())), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	defer trace.Close()
	var sendMu sync.Mutex
	var gamePDUMu sync.Mutex
	record := func(direction string, f aries.Frame) {
		fmt.Fprintf(trace, "%s type=%#x tag=%#x length=%d\n%s", direction, f.Type, f.Tag, len(f.Payload), hex.Dump(f.Payload))
		log.Printf("%s type=%#x length=%d", direction, f.Type, len(f.Payload))
	}
	send := func(f aries.Frame) error {
		sendMu.Lock()
		defer sendMu.Unlock()
		record("send", f)
		return aries.Write(conn, f)
	}
	sendGamePDU := func(p protocol.PDU) error {
		gamePDUMu.Lock()
		defer gamePDUMu.Unlock()
		fragments, err := protocol.FragmentPDU(p, protocol.ClientBufferSize)
		if err != nil {
			return err
		}
		if len(fragments) > 1 {
			log.Printf("Fragmenting game PDU %#x (%d encoded bytes) into %d SPLIT_BUFFER_PDU frames", p.ID, len(protocol.Encode(p)), len(fragments))
		}
		for _, fragment := range fragments {
			if err := send(aries.Frame{Type: aries.Data, Payload: protocol.Encode(fragment)}); err != nil {
				return err
			}
		}
		return nil
	}
	session := &liveSession{sendGamePDU: sendGamePDU}
	if hub != nil {
		defer hub.remove(session)
	}
	log.Printf("Client connected: %s", conn.RemoteAddr())
	configPending := false
	activePilot := ""
	planetStateSent := false
	var lastOwnerSnapshot []byte
	var ownerSnapshotSequence uint32
	if err := send(aries.Frame{Type: aries.LoginRequest}); err != nil {
		return err
	}
	for {
		conn.SetDeadline(time.Now().Add(5 * time.Minute))
		f, err := aries.Read(conn)
		if err != nil {
			return err
		}
		record("recv", f)
		switch f.Type {
		case aries.Login:
			if err := sendGamePDU(protocol.HostGreeting()); err != nil {
				return err
			}
		case aries.Data:
			messages, err := protocol.Decode(f.Payload)
			if err != nil {
				return err
			}
			for _, p := range messages {
				log.Printf("Game PDU id=%#x body=%s", p.ID, hex.EncodeToString(p.Body))
				switch p.ID {
				case protocol.ClientOnline:
					// Send the local pilot and profile before beginning config sync.
					if err := sendGamePDU(protocol.PlayerUpdate()); err != nil {
						return err
					}
					if err := sendGamePDU(protocol.ProfileUpdate()); err != nil {
						return err
					}
					if err := sendGamePDU(protocol.ConfigurationUpdate(false, 0)); err != nil {
						return err
					}
					configPending = true
				case protocol.StaticConfigResponse:
					if !configPending {
						log.Printf("Ignoring unexpected static-config response")
						continue
					}
					configPending = false
					if err := sendGamePDU(protocol.ConfigurationSynchronized()); err != nil {
						return err
					}
				case protocol.CreateCharacter:
					callSign, err := protocol.ParseCreateCharacterRequest(p.Body)
					if err != nil {
						log.Printf("Ignoring invalid create-character request: %v", err)
						continue
					}
					pilot, err := world.CreatePilot(callSign)
					if err != nil {
						log.Printf("Rejecting create-character request: %v", err)
						continue
					}
					activePilot = pilot.CallSign
					log.Printf("Create-character request for call sign %q; saved at planet %d with %d rank points", activePilot, pilot.Location.PlanetID, pilot.RankPoints)
					// The zero result uses the completion path in the client's
					// CREATE_CHARACTER_FAILED handler and reached the map shell locally.
					// Keep it marked experimental until persistence is verified.
					if err := sendGamePDU(protocol.CreateCharacterCompletion()); err != nil {
						return err
					}
					// Keep the character-creation handshake at the last known safe
					// shape. A post-create player/profile refresh with an invented ID
					// crashes this client before it reaches Solaris.
				case protocol.JoinRoom:
					id, password, err := protocol.ParseJoinRoomRequest(p.Body)
					if err != nil {
						log.Printf("Invalid JOIN_ROOM: %v", err)
						continue
					}
					if activePilot == "" || id.Host != "local" || password != "" {
						if err := sendGamePDU(protocol.JoinRoomFailure("Invalid mission join")); err != nil {
							return err
						}
						continue
					}
					currentPilot, _ := world.Pilot(activePilot)
					// The native Back button rejoins the parent city using
					// JOIN_ROOM; it does not issue EXIT_ROOM.
					if currentPilot.Location.PlanetID == state.SolarisPlanetID && currentPilot.Location.CityID != 0 && id == solarisLobbyRoomID(currentPilot.Location.CityID) {
						if err := world.LeaveRoom(activePilot); err != nil {
							return err
						}
						pilot, _ := world.Pilot(activePilot)
						if hub != nil {
							hub.mu.Lock()
							previous := hub.sessions[session]
							hub.mu.Unlock()
							hub.publishMissionDeparture(session, previous, pilot)
						}
						log.Printf("Pilot %q returned to Solaris staging", activePilot)
						continue
					}
					if err := world.JoinAvailableRoom(activePilot, int(id.Number)); err != nil {
						if err := sendGamePDU(protocol.JoinRoomFailure(err.Error())); err != nil {
							return err
						}
						continue
					}
					pilot, _ := world.Pilot(activePilot)
					rooms, err := solarisMissionRooms(world, pilot)
					if err != nil {
						return err
					}
					for _, room := range rooms {
						if room.ID == id {
							log.Printf("Pilot %q joined mission %d side %d seat %d", activePilot, id.Number, pilot.Location.Side, pilot.Location.Seat)
							if hub != nil {
								hub.publishMissionJoin(session, pilot, room, missionCombat(world, room), missionSeatUpdates(world, int(room.ID.Number)))
								if err := sendGamePDU(missionCountdown(world, pilot.Location.RoomID, time.Now())); err != nil {
									return err
								}
							}
						}
					}

				case protocol.ListRooms:
					if err := sendGamePDU(protocol.ListRoomsResponse()); err != nil {
						return err
					}
				case protocol.ExitRoom:
					roomName, err := protocol.ParseExitRoomRequest(p.Body)
					if err != nil {
						log.Printf("Ignoring invalid exit-room request: %v", err)
						continue
					}
					if activePilot != "" {
						if err := world.LeaveRoom(activePilot); err != nil {
							log.Printf("Could not remove %q from its mission room: %v", activePilot, err)
							continue
						}
					}
					log.Printf("Exit-room request for pilot %q and room %q", activePilot, roomName)
					if err := sendGamePDU(protocol.ExitRoomUpdate(roomName)); err != nil {
						return err
					}
					if hub != nil && activePilot != "" {
						pilot, _ := world.Pilot(activePilot)
						hub.mu.Lock()
						previous := hub.sessions[session]
						hub.mu.Unlock()
						hub.publishMissionDeparture(session, previous, pilot)
					}
				case protocol.JoinPlanet:
					planetID, password, err := protocol.ParseJoinPlanetRequest(p.Body)
					if err != nil {
						log.Printf("Ignoring invalid join-planet request: %v", err)
						continue
					}
					log.Printf("Join-planet request for planet %d (password supplied: %t)", planetID, password != "")
					if activePilot == "" {
						log.Printf("Cannot apply planet join before a pilot is identified")
						continue
					}
					if err := world.JoinPlanet(activePilot, int(planetID)); err != nil {
						log.Printf("Planet join rejected for %q: %v", activePilot, err)
						if err := sendGamePDU(protocol.JoinPlanetFailure(1, err.Error())); err != nil {
							return err
						}
						continue
					}
					log.Printf("Pilot %q is now at planet %d", activePilot, planetID)
					// Publish the pilot's current planet roster to the joining client
					// and other connected clients already on the planet.
					if hub == nil {
						if err := sendGamePDU(protocol.OccupantUpdateFor(activePilot)); err != nil {
							return err
						}
					} else {
						pilot, ok := world.Pilot(activePilot)
						if !ok {
							log.Printf("Cannot publish arrival for unknown pilot %q", activePilot)
							continue
						}
						hub.mu.Lock()
						previous := hub.sessions[session]
						hub.mu.Unlock()
						if previous.location.RoomID != 0 {
							hub.publishMissionDeparture(session, previous, pilot)
						} else {
							hub.publishPlanetJoin(session, pilot)
						}
						if hub.world != nil {
							hub.publishRoomLists()
						} else if hub.planetRoomType >= 0 && planetID == state.SolarisPlanetID {
							rooms, err := solarisMissionRooms(world, pilot)
							if err != nil {
								return err
							}
							for _, room := range rooms {
								if err := sendGamePDU(protocol.ReadyRoomAdded(room, missionSeatLites(world, room.ID.Number))); err != nil {
									return err
								}
							}
						}
					}
					log.Printf("Published occupant arrival for pilot %q on planet %d", activePilot, planetID)
					if emptySolarisRoomListAfterJoin && planetID == state.SolarisPlanetID {
						if err := sendGamePDU(protocol.EmptyReadyRoomList()); err != nil {
							return err
						}
						log.Printf("Sent experimental empty Solaris ready-room list after occupant arrival")
					}
					if redirectAddress != "" {
						redirect, err := aries.RedirectFrame(redirectAddress, redirectAddress, "")
						if err != nil {
							return fmt.Errorf("build experimental planet redirect: %w", err)
						}
						if err := send(redirect); err != nil {
							return err
						}
						log.Printf("Sent experimental planet redirect to %s", redirectAddress)
						return nil
					}
				case protocol.ChatMessageID:
					if hub == nil {
						if activePilot == "" {
							log.Printf("Ignoring chat before a pilot is identified")
							continue
						}
						canonical, err := protocol.ChatMessageForDisplay(p, activePilot)
						if err != nil {
							log.Printf("Ignoring malformed chat from %q: %v", activePilot, err)
							continue
						}
						if err := sendGamePDU(canonical); err != nil {
							return err
						}
						continue
					}
					hub.publishPlanetChat(session, p)
				case protocol.CombatDataID:
					if hub != nil {
						hub.noteCombatPrepared(session, p.Body)
						hub.relayCombatPosition(session, p.Body)
					}
				case protocol.ReadyStateChangeID:
					options, err := protocol.ParseSeatOptionsRequest(p.Body)
					if err != nil {
						log.Printf("Ignoring malformed seat options from %q: %v", activePilot, err)
						continue
					}
					if options.Option == protocol.SeatOptionState && options.Seat == 0 && options.MechID == 0 && options.State == 3 {
						if activePilot == "" {
							log.Printf("Ignoring auto-launch request before a pilot is identified")
							continue
						}
						pilot, ok := world.Pilot(activePilot)
						if !ok || pilot.Location.RoomID == 0 {
							log.Printf("Ignoring auto-launch request from %q outside a mission", activePilot)
							continue
						}
						started, err := world.StartExpiredBattles(time.Now())
						if err != nil {
							log.Printf("Starting expired missions: %v", err)
							continue
						}
						launched := false
						for _, room := range started {
							if room.ID != pilot.Location.RoomID {
								continue
							}
							launched = true
							if hub != nil {
								hub.launchMission(room.ID)
							} else if err := sendGamePDU(protocol.EnterCombat()); err != nil {
								return err
							}
							log.Printf("Mission %d entered combat after native auto-launch request from %q", room.ID, activePilot)
						}
						if !launched {
							log.Printf("Ignoring early or duplicate auto-launch request from %q in mission %d", activePilot, pilot.Location.RoomID)
						}
						continue
					}
					if options.Option == protocol.SeatOptionMech && options.Seat == 0 {
						if err := world.SelectMissionMech(activePilot, options.MechID); err != nil {
							log.Printf("Ignoring Mech selection from %q: %v", activePilot, err)
							continue
						}
						pilot, _ := world.Pilot(activePilot)
						for _, seat := range missionSeatUpdates(world, pilot.Location.RoomID) {
							if strings.EqualFold(seat.CallSign, activePilot) {
								if hub != nil {
									hub.publishMissionSeatUpdate(pilot, seat)
								} else if err := sendGamePDU(protocol.SeatOptionsUpdate(seat, 0)); err != nil {
									return err
								}
								break
							}
						}
						log.Printf("Pilot %q selected native Mech %d in mission %d", activePilot, options.MechID, pilot.Location.RoomID)
						continue
					}
					ready, err := protocol.ParseReadyStateChange(p.Body)
					if err != nil {
						log.Printf("Ignoring unsupported ready-state request from %q: %v", activePilot, err)
						continue
					}
					if activePilot == "" {
						log.Printf("Ignoring ready-state request before a pilot is identified")
						continue
					}
					if err := world.SetReady(activePilot, ready, time.Now()); err != nil {
						log.Printf("Ignoring ready-state request from %q: %v", activePilot, err)
						continue
					}
					pilot, ok := world.Pilot(activePilot)
					if !ok {
						continue
					}
					for _, seat := range missionSeatUpdates(world, pilot.Location.RoomID) {
						if strings.EqualFold(seat.CallSign, activePilot) {
							if hub != nil {
								hub.publishMissionSeatUpdate(pilot, seat)
								roomInfo := protocol.RoomInfo{ID: protocol.RoomID{Number: uint32(pilot.Location.RoomID), Host: "local"}}
								hub.publishMissionCombatUpdate(pilot, missionCombat(world, roomInfo))
								hub.publishMissionCombatUpdate(pilot, missionCountdown(world, pilot.Location.RoomID, time.Now()))
							} else if err := sendGamePDU(protocol.SeatOptionsUpdate(seat, 0)); err != nil {
								return err
							}
							log.Printf("Pilot %q is %s in mission %d", activePilot, map[bool]string{true: "ready", false: "not ready"}[ready], pilot.Location.RoomID)
							break
						}
					}
				case protocol.RequestPlanetStates:
					planetID, err := protocol.ParsePlanetStatesRequest(p.Body)
					if err != nil {
						log.Printf("Ignoring invalid planet-states request: %v", err)
						continue
					}
					log.Printf("Planet-states request for planet %d", planetID)
					if planetID != 0 {
						continue
					}
					if planetStateReply != nil {
						if !planetStateSent {
							if err := sendGamePDU(*planetStateReply); err != nil {
								return err
							}
							planetStateSent = true
							log.Printf("Sent one experimental planet-state snapshot in this session")
						}
						continue
					}
					// The second entry scalar drives House-colored territory fills.
					// Send again only when ownership changes to avoid needless redraws.
					currentWorld := world.Snapshot()
					state := planetStateOwnerSnapshot(currentWorld, 0)
					if bytes.Equal(state.Body, lastOwnerSnapshot) {
						continue
					}
					baselineBody := state.Body
					if lastOwnerSnapshot != nil {
						ownerSnapshotSequence++
						state = planetStateOwnerSnapshot(currentWorld, ownerSnapshotSequence)
					}
					if err := sendGamePDU(state); err != nil {
						return err
					}
					lastOwnerSnapshot = append(lastOwnerSnapshot[:0], baselineBody...)
					log.Printf("Sent House-colored planet-state snapshot (sequence %d)", ownerSnapshotSequence)
				case protocol.RequestLobbyRooms:
					planetID, err := protocol.ParseLobbyRoomsRequest(p.Body)
					if err != nil {
						log.Printf("Ignoring malformed lobby-rooms request: %v", err)
						continue
					}
					log.Printf("Lobby-rooms request for planet %d", planetID)
					if err := sendGamePDU(protocol.EmptyReadyRoomList()); err != nil {
						return err
					}
				default:
					if response, ok := protocol.StartupSettingResponse(p.ID); ok {
						if err := sendGamePDU(response); err != nil {
							return err
						}
					}
				}
			}
		default:
			log.Printf("Unimplemented transport message %#x", f.Type)
		}
	}
}
