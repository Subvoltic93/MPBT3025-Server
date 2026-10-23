package state

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

type Store struct {
	mu    sync.RWMutex
	path  string
	world World
}

type MatchResult struct {
	MatchID              string
	RoomID               int
	WinningSide          int
	AttackingSide        int
	AttackingHouse       HouseID
	ParticipantCallSigns []string
}

// Open loads the saved campaign or creates a prototype world at path.
func Open(path string) (*Store, error) {
	data, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("read world state: %w", err)
	}
	var world World
	firstRun := errors.Is(err, os.ErrNotExist)
	ownershipMigrated := false
	if firstRun {
		world, err = DefaultWorld()
		if err != nil {
			return nil, err
		}
	} else if err := json.Unmarshal(data, &world); err != nil {
		return nil, fmt.Errorf("decode world state: %w", err)
	} else {
		ownershipMigrated, err = world.migrateLegacySyntheticOwnership()
		if err != nil {
			return nil, fmt.Errorf("migrate initial planet ownership: %w", err)
		}
	}
	playerIDsMigrated, err := migratePlayerIDs(&world)
	if err != nil {
		return nil, fmt.Errorf("migrate pilot IDs: %w", err)
	}
	solarisRoomsMigrated := false
	for cityID := 1; cityID <= SolarisCityCount; cityID++ {
		solarisRoomsMigrated = world.ensureSolarisRoomSlots(cityID, true) || solarisRoomsMigrated
		solarisRoomsMigrated = world.ensureSolarisRoomSlots(cityID, false) || solarisRoomsMigrated
	}
	if err := world.Validate(); err != nil {
		return nil, fmt.Errorf("invalid world state: %w", err)
	}
	store := &Store{path: path, world: world}
	if firstRun || ownershipMigrated || playerIDsMigrated || solarisRoomsMigrated {
		if err := store.saveLocked(); err != nil {
			return nil, err
		}
	}
	return store, nil
}

// ResetSessionPresence is called once by the server at startup. TCP sessions
// cannot survive a restart; saved pilot progression and lobby choices can.
func (s *Store) ResetSessionPresence() error {
	return s.update(func(world *World) error {
		for key, pilot := range world.Pilots {
			pilot.Location.RoomID, pilot.Location.Side, pilot.Location.Seat = 0, 0, 0
			world.Pilots[key] = pilot
		}
		for id, room := range world.Rooms {
			room.Seats = [2][LanceSeatCount]Seat{}
			room.CountdownEnds = time.Time{}
			if room.Status != RoomComplete {
				room.Status = RoomOpen
			}
			world.Rooms[id] = room
		}
		return nil
	})
}

func migratePlayerIDs(world *World) (bool, error) {
	if world.Pilots == nil {
		world.Pilots = make(map[string]Pilot)
	}
	changed := false
	var maxID uint32
	for _, pilot := range world.Pilots {
		if pilot.PlayerID > maxID {
			maxID = pilot.PlayerID
		}
	}
	if world.NextPlayerID == 0 || world.NextPlayerID <= maxID {
		if maxID == ^uint32(0) {
			return false, errors.New("player ID space is exhausted")
		}
		world.NextPlayerID = maxID + 1
		changed = true
	}
	callSigns := make([]string, 0, len(world.Pilots))
	for key := range world.Pilots {
		callSigns = append(callSigns, key)
	}
	sort.Strings(callSigns)
	for _, key := range callSigns {
		pilot := world.Pilots[key]
		if pilot.PlayerID != 0 {
			continue
		}
		if world.NextPlayerID == ^uint32(0) {
			return false, errors.New("player ID space is exhausted")
		}
		pilot.PlayerID = world.NextPlayerID
		world.NextPlayerID++
		world.Pilots[key] = pilot
		changed = true
	}
	return changed, nil
}

func (s *Store) Snapshot() World {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return cloneWorld(s.world)
}

func (s *Store) CreatePilot(callSign string) (Pilot, error) {
	callSign = strings.TrimSpace(callSign)
	if len(callSign) < 4 || len(callSign) > 15 {
		return Pilot{}, fmt.Errorf("call sign must be 4–15 bytes, got %d", len(callSign))
	}
	var created Pilot
	err := s.update(func(world *World) error {
		key := pilotKey(callSign)
		if existing, exists := world.Pilots[key]; exists {
			created = existing
			return nil
		}
		if world.NextPlayerID == 0 || world.NextPlayerID == ^uint32(0) {
			return errors.New("player ID space is exhausted")
		}
		created = Pilot{
			CallSign:   callSign,
			PlayerID:   world.NextPlayerID,
			OwnedMechs: []string{"Commando-4D"},
			CreatedAt:  time.Now().UTC(),
		}
		world.NextPlayerID++
		world.Pilots[key] = created
		return nil
	})
	return created, err
}

func (s *Store) Pilot(callSign string) (Pilot, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	pilot, ok := s.world.Pilots[pilotKey(callSign)]
	return pilot, ok
}

func (s *Store) Planet(planetID int) (Planet, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	planet, ok := s.world.Planets[planetID]
	return planet, ok
}

func (s *Store) JoinPlanet(callSign string, planetID int) error {
	return s.update(func(world *World) error {
		pilot, ok := world.Pilots[pilotKey(callSign)]
		if !ok {
			return fmt.Errorf("unknown pilot %q", callSign)
		}
		planet, ok := world.Planets[planetID]
		if !ok {
			return fmt.Errorf("unknown planet %d", planetID)
		}
		if !pilot.Graduated && planetID != SolarisPlanetID {
			return errors.New("cadet pilots may only join Solaris")
		}
		if planetID != SolarisPlanetID {
			if pilot.House == "" {
				return errors.New("pilot must choose a House before joining the Inner Sphere")
			}
			if planet.Owner != pilot.House && !isAdjacentToHouse(*world, planet, pilot.House) {
				return errors.New("planet is not controlled by or adjacent to the pilot's House")
			}
		}
		removePilotFromRooms(world, callSign)
		previousLocation := pilot.Location
		location := Location{PlanetID: planetID}
		if len(planet.Cities) > 0 {
			// The manual sends returning graduates through a sixteen-city picker.
			// Keep them at the planet level until they choose a city; cadets are
			// assigned to a city lobby as soon as they enter Solaris.
			if !pilot.Graduated {
				preferredCityID := 0
				if pilot.Location.PlanetID == planetID {
					preferredCityID = pilot.Location.CityID
				}
				city, err := availableSolarisCity(*world, preferredCityID, pilot.CallSign)
				if err != nil {
					return err
				}
				location.CityID = city.ID
			} else if previousLocation.PlanetID == planetID {
				location.CityID = previousLocation.CityID
			} else {
				origin := previousLocation
				pilot.SolarisCityPickerOrigin = &origin
			}
		} else {
			pilot.SolarisCityPickerOrigin = nil
		}
		pilot.Location = location
		world.Pilots[pilotKey(callSign)] = pilot
		return nil
	})
}

// SolarisCityStatus is the returning-pilot city picker data. Occupancy counts
// stationed 'Mechs, and Full is true when no additional 'Mech fits in the city.
type SolarisCityStatus struct {
	ID            int    `json:"id"`
	Name          string `json:"name"`
	OccupiedMechs int    `json:"occupied_mechs"`
	MaxMechs      int    `json:"max_mechs"`
	Full          bool   `json:"full"`
}

// SolarisCityDirectory reports the sixteen Solaris cities in display order.
// The city list is static world data; occupancy comes from saved pilot stations.
func (s *Store) SolarisCityDirectory() []SolarisCityStatus {
	s.mu.RLock()
	defer s.mu.RUnlock()
	planet := s.world.Planets[SolarisPlanetID]
	cities := make([]SolarisCityStatus, 0, len(planet.Cities))
	for _, city := range planet.Cities {
		occupied := cityMechOccupancy(s.world, city.ID, "")
		capacity := cityMechCapacity(city)
		cities = append(cities, SolarisCityStatus{
			ID:            city.ID,
			Name:          city.Name,
			OccupiedMechs: occupied,
			MaxMechs:      capacity,
			Full:          occupied >= capacity,
		})
	}
	return cities
}

// JoinSolarisCity places a graduated pilot in a selected Solaris city. Cadets
// use the automatically assigned city on their first planet join.
func (s *Store) JoinSolarisCity(callSign string, cityID int) error {
	return s.update(func(world *World) error {
		pilot, ok := world.Pilots[pilotKey(callSign)]
		if !ok {
			return fmt.Errorf("unknown pilot %q", callSign)
		}
		if !pilot.Graduated {
			return errors.New("cadets are assigned to a Solaris city automatically")
		}
		if pilot.RankPoints < GraduationRankPoints {
			return errors.New("graduated Solaris travel requires the Graduation Medal")
		}
		if pilot.Location.PlanetID != SolarisPlanetID || pilot.Location.CityID != 0 || pilot.SolarisCityPickerOrigin == nil {
			return errors.New("pilot has no pending Solaris city selection")
		}
		pilot.SolarisGraduationMedal = true // Legacy graduated saves imply the award.
		city, ok := solarisCity(*world, cityID)
		if !ok {
			return fmt.Errorf("Solaris has no city %d", cityID)
		}
		if cityMechOccupancy(*world, cityID, pilot.CallSign)+pilotMechCount(*world, pilot.CallSign) > cityMechCapacity(city) {
			return errors.New("Solaris city is full")
		}
		removePilotFromRooms(world, pilot.CallSign)
		// The picker can only be completed after the pilot has returned from
		// elsewhere. Replace that pending location in one step so room, sector,
		// and seat IDs from the origin cannot leak into the selected city.
		pilot.Location = Location{PlanetID: SolarisPlanetID, CityID: cityID}
		pilot.SolarisCityPickerOrigin = nil
		world.Pilots[pilotKey(callSign)] = pilot
		return nil
	})
}

// CancelSolarisCityPicker is the documented Cancel path for a returning
// graduate. It restores the location held before opening the city selection.
func (s *Store) CancelSolarisCityPicker(callSign string) error {
	return s.update(func(world *World) error {
		pilot, ok := world.Pilots[pilotKey(callSign)]
		if !ok {
			return fmt.Errorf("unknown pilot %q", callSign)
		}
		if !pilot.Graduated || pilot.Location.PlanetID != SolarisPlanetID || pilot.Location.CityID != 0 || pilot.SolarisCityPickerOrigin == nil {
			return errors.New("pilot has no pending Solaris city selection")
		}
		pilot.Location = *pilot.SolarisCityPickerOrigin
		pilot.SolarisCityPickerOrigin = nil
		world.Pilots[pilotKey(callSign)] = pilot
		return nil
	})
}

// RoomsForSolarisCity returns open or counting-down missions that match the
// caller's cadet/graduated status. Graduates cannot join cadet missions, and
// cadets cannot join experienced-pilot practice rooms.
func (s *Store) RoomsForSolarisCity(callSign string) ([]MissionRoom, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	pilot, ok := s.world.Pilots[pilotKey(callSign)]
	if !ok {
		return nil, fmt.Errorf("unknown pilot %q", callSign)
	}
	if pilot.Location.PlanetID != SolarisPlanetID || pilot.Location.CityID == 0 {
		return nil, errors.New("pilot is not in a Solaris city lobby")
	}
	rooms := make([]MissionRoom, 0)
	for _, room := range s.world.Rooms {
		if room.PlanetID != SolarisPlanetID || room.CityID != pilot.Location.CityID || room.Status == RoomInCombat || room.Status == RoomComplete {
			continue
		}
		if room.CadetOnly == !pilot.Graduated {
			rooms = append(rooms, room)
		}
	}
	sort.Slice(rooms, func(i, j int) bool { return rooms[i].ID < rooms[j].ID })
	return rooms, nil
}

func (s *Store) JoinSector(callSign string, sectorID int) error {
	return s.update(func(world *World) error {
		pilot, ok := world.Pilots[pilotKey(callSign)]
		if !ok {
			return fmt.Errorf("unknown pilot %q", callSign)
		}
		planet, ok := world.Planets[pilot.Location.PlanetID]
		if !ok || len(planet.Sectors) == 0 {
			return errors.New("current planet has no strategic sectors")
		}
		for _, sector := range planet.Sectors {
			if sector.ID == sectorID {
				pilot.Location.SectorID = sectorID
				pilot.Location.CityID = 0
				pilot.SolarisCityPickerOrigin = nil
				world.Pilots[pilotKey(callSign)] = pilot
				return nil
			}
		}
		return fmt.Errorf("planet %q has no sector %d", planet.Name, sectorID)
	})
}

func (s *Store) ChooseHouse(callSign string, houseID HouseID) error {
	return s.update(func(world *World) error {
		pilot, ok := world.Pilots[pilotKey(callSign)]
		if !ok {
			return fmt.Errorf("unknown pilot %q", callSign)
		}
		if !pilot.Graduated || pilot.RankPoints < GraduationRankPoints {
			return errors.New("pilot needs the Solaris Graduation Medal before choosing a House")
		}
		if pilot.House != "" {
			return errors.New("pilot has already chosen a House")
		}
		pilot.SolarisGraduationMedal = true // Legacy graduated saves imply the award.
		if !knownHouse(*world, houseID) {
			return fmt.Errorf("unknown House %q", houseID)
		}
		pilot.House = houseID
		pilot.Location = Location{}
		pilot.SolarisCityPickerOrigin = nil
		world.Pilots[pilotKey(callSign)] = pilot
		return nil
	})
}

func (s *Store) RoomsForPlanet(planetID int) []MissionRoom {
	s.mu.RLock()
	defer s.mu.RUnlock()
	rooms := make([]MissionRoom, 0)
	for _, room := range s.world.Rooms {
		if room.PlanetID == planetID && room.Status != RoomInCombat && room.Status != RoomComplete {
			rooms = append(rooms, room)
		}
	}
	sort.Slice(rooms, func(i, j int) bool { return rooms[i].ID < rooms[j].ID })
	return rooms
}

func (s *Store) CreateRoom(callSign, name, mission string) (MissionRoom, error) {
	name = strings.TrimSpace(name)
	mission = strings.TrimSpace(mission)
	if name == "" || mission == "" {
		return MissionRoom{}, errors.New("room name and mission are required")
	}
	var created MissionRoom
	err := s.update(func(world *World) error {
		pilot, ok := world.Pilots[pilotKey(callSign)]
		if !ok {
			return fmt.Errorf("unknown pilot %q", callSign)
		}
		if pilot.Location.PlanetID == 0 || (pilot.Location.CityID == 0 && pilot.Location.SectorID == 0) {
			return errors.New("pilot must be in a Solaris city or strategic sector to create a room")
		}
		created = MissionRoom{
			ID:        world.NextRoomID,
			PlanetID:  pilot.Location.PlanetID,
			CityID:    pilot.Location.CityID,
			SectorID:  pilot.Location.SectorID,
			Name:      name,
			Mission:   mission,
			CadetOnly: pilot.Location.PlanetID == SolarisPlanetID && !pilot.Graduated,
			Status:    RoomOpen,
		}
		world.Rooms[created.ID] = created
		world.NextRoomID++
		return nil
	})
	return created, err
}

func (s *Store) JoinRoom(callSign string, roomID, side, seatIndex int) error {
	return s.update(func(world *World) error { return joinRoom(world, callSign, roomID, side, seatIndex) })
}

// JoinAvailableRoom assigns the least populated lance while holding the store lock.
func (s *Store) JoinAvailableRoom(callSign string, roomID int) error {
	return s.update(func(world *World) error {
		room, ok := world.Rooms[roomID]
		if !ok {
			return fmt.Errorf("unknown mission room %d", roomID)
		}
		counts := [2]int{}
		for side, seats := range room.Seats {
			for _, seat := range seats {
				if strings.EqualFold(seat.CallSign, callSign) {
					return nil
				}
				if seat.CallSign != "" || seat.AI {
					counts[side]++
				}
			}
		}
		order := [2]int{0, 1}
		if counts[1] < counts[0] {
			order = [2]int{1, 0}
		}
		for _, side := range order {
			for index, seat := range room.Seats[side] {
				if seat.CallSign == "" && !seat.AI {
					return joinRoom(world, callSign, roomID, side, index)
				}
			}
		}
		return errors.New("mission room is full")
	})
}

func joinRoom(world *World, callSign string, roomID, side, seatIndex int) error {
	pilot, ok := world.Pilots[pilotKey(callSign)]
	if !ok {
		return fmt.Errorf("unknown pilot %q", callSign)
	}
	room, ok := world.Rooms[roomID]
	if !ok {
		return fmt.Errorf("unknown mission room %d", roomID)
	}
	if room.Status != RoomOpen && room.Status != RoomCountdown {
		return errors.New("mission room is not accepting pilots")
	}
	if room.PlanetID == SolarisPlanetID && room.CadetOnly == pilot.Graduated {
		return errors.New("Solaris cadets and graduated pilots use separate missions")
	}
	if room.PlanetID != pilot.Location.PlanetID || room.CityID != pilot.Location.CityID || room.SectorID != pilot.Location.SectorID {
		return errors.New("pilot and room are in different lobbies")
	}
	if side < 0 || side >= len(room.Seats) || seatIndex < 0 || seatIndex >= LanceSeatCount {
		return errors.New("seat is outside the two four-pilot lances")
	}
	if occupant := room.Seats[side][seatIndex].CallSign; occupant != "" && !strings.EqualFold(occupant, callSign) {
		return errors.New("seat is occupied")
	}
	removePilotFromRooms(world, callSign)
	room = world.Rooms[roomID]
	room.Seats[side][seatIndex] = Seat{CallSign: pilot.CallSign}
	if room.Status != RoomCountdown {
		room.Status = RoomOpen
		room.CountdownEnds = time.Time{}
	}
	pilot.Location.RoomID = room.ID
	pilot.Location.Side = side
	pilot.Location.Seat = seatIndex
	world.Rooms[room.ID] = room
	world.Pilots[pilotKey(callSign)] = pilot
	if room.PlanetID == SolarisPlanetID {
		world.ensureSolarisRoomSlots(room.CityID, room.CadetOnly)
	}
	return nil
}

func (s *Store) LeaveRoom(callSign string) error {
	return s.update(func(world *World) error {
		pilot, ok := world.Pilots[pilotKey(callSign)]
		if !ok {
			return fmt.Errorf("unknown pilot %q", callSign)
		}
		if pilot.Location.RoomID == 0 {
			return nil
		}
		removePilotFromRooms(world, callSign)
		pilot.Location.RoomID = 0
		pilot.Location.Side = 0
		pilot.Location.Seat = 0
		world.Pilots[pilotKey(callSign)] = pilot
		return nil
	})
}

type MechSpec struct {
	Model       string
	WeightTons  int
	WeightClass int
	Price       int
}

var prototypeMechCatalog = map[string]MechSpec{
	"Commando-4D":        {Model: "Commando-4D", WeightTons: 25, WeightClass: 1, Price: 2_000_000},
	"Jenner JR7-D":       {Model: "Jenner JR7-D", WeightTons: 35, WeightClass: 1, Price: 3_000_000},
	"Shadow Hawk SHD-2H": {Model: "Shadow Hawk SHD-2H", WeightTons: 55, WeightClass: 2, Price: 5_000_000},
	"Wolverine WVR-6R":   {Model: "Wolverine WVR-6R", WeightTons: 55, WeightClass: 2, Price: 5_500_000},
	"Warhammer WHM-6R":   {Model: "Warhammer WHM-6R", WeightTons: 70, WeightClass: 3, Price: 7_000_000},
	"Atlas AS7-D":        {Model: "Atlas AS7-D", WeightTons: 100, WeightClass: 4, Price: 10_000_000},
}

func (s *Store) BuyMech(callSign, model string) error {
	return s.update(func(world *World) error {
		pilot, ok := world.Pilots[pilotKey(callSign)]
		if !ok {
			return fmt.Errorf("unknown pilot %q", callSign)
		}
		if pilot.House == "" || !pilot.Graduated {
			return errors.New("cadets cannot buy 'Mechs before graduation and House selection")
		}
		spec, ok := prototypeMechCatalog[model]
		if !ok {
			return fmt.Errorf("unknown 'Mech model %q", model)
		}
		planet, ok := world.Planets[pilot.Location.PlanetID]
		if !ok || spec.WeightClass > planet.MaxMechWeightClass {
			return errors.New("current planet does not allow this 'Mech weight class")
		}
		if pilot.Credits < spec.Price {
			return fmt.Errorf("not enough C-bills: need %d, have %d", spec.Price, pilot.Credits)
		}
		for _, owned := range pilot.OwnedMechs {
			if owned == model {
				return fmt.Errorf("pilot already owns %q", model)
			}
		}
		pilot.Credits -= spec.Price
		pilot.OwnedMechs = append(pilot.OwnedMechs, model)
		world.Pilots[pilotKey(callSign)] = pilot
		return nil
	})
}

// PaySalary credits a prototype salary once per named campaign period. The
// manual ties salary to House-owned planet economies; the multiplier is an
// editable approximation until the historical salary table is recovered.
func (s *Store) PaySalary(callSign, period string) (int, error) {
	period = strings.TrimSpace(period)
	if period == "" {
		return 0, errors.New("salary period is required")
	}
	paid := 0
	err := s.update(func(world *World) error {
		pilot, ok := world.Pilots[pilotKey(callSign)]
		if !ok {
			return fmt.Errorf("unknown pilot %q", callSign)
		}
		if pilot.House == "" {
			return errors.New("pilot must choose a House before receiving campaign salary")
		}
		claimKey := pilotKey(callSign) + ":" + period
		if world.SalaryClaims[claimKey] {
			return fmt.Errorf("salary for period %q was already paid", period)
		}
		paid = 100
		for _, planet := range world.Planets {
			if planet.Owner == pilot.House {
				paid += planet.EconomyValue * 25
			}
		}
		pilot.Credits += paid
		world.Pilots[pilotKey(callSign)] = pilot
		world.SalaryClaims[claimKey] = true
		return nil
	})
	return paid, err
}

// SelectMissionMech records the native client selection for its own occupied
// seat. This wire ID is distinct from the prototype economy's model names.
func (s *Store) SelectMissionMech(callSign string, mechID uint32) error {
	return s.update(func(world *World) error {
		pilot, ok := world.Pilots[pilotKey(callSign)]
		if !ok || pilot.Location.RoomID == 0 {
			return errors.New("pilot is not seated in a mission room")
		}
		room, ok := world.Rooms[pilot.Location.RoomID]
		if !ok || room.Status == RoomInCombat || room.Status == RoomComplete {
			return errors.New("mission room is not accepting Mech selections")
		}
		if pilot.Location.Side < 0 || pilot.Location.Side >= len(room.Seats) || pilot.Location.Seat < 0 || pilot.Location.Seat >= LanceSeatCount {
			return errors.New("pilot's mission seat is outside the room")
		}
		seat := &room.Seats[pilot.Location.Side][pilot.Location.Seat]
		if !strings.EqualFold(seat.CallSign, pilot.CallSign) {
			return errors.New("pilot's mission seat is not occupied by that pilot")
		}
		seat.MechID = mechID
		world.Rooms[room.ID] = room
		return nil
	})
}

func (s *Store) SetReady(callSign string, ready bool, now time.Time) error {
	return s.update(func(world *World) error {
		pilot, ok := world.Pilots[pilotKey(callSign)]
		if !ok || pilot.Location.RoomID == 0 {
			return errors.New("pilot is not seated in a mission room")
		}
		room, ok := world.Rooms[pilot.Location.RoomID]
		if !ok || room.Status == RoomInCombat || room.Status == RoomComplete {
			return errors.New("mission room is not accepting readiness changes")
		}
		if pilot.Location.Side < 0 || pilot.Location.Side >= len(room.Seats) || pilot.Location.Seat < 0 || pilot.Location.Seat >= LanceSeatCount {
			return errors.New("pilot's mission seat is outside the room")
		}
		seat := &room.Seats[pilot.Location.Side][pilot.Location.Seat]
		if !strings.EqualFold(seat.CallSign, pilot.CallSign) {
			return errors.New("pilot's mission seat is not occupied by that pilot")
		}
		seat.Ready = ready
		if !ready {
			if roomHasReadyPlayers(room) {
				if room.Status != RoomCountdown {
					room.Status = RoomCountdown
					room.CountdownEnds = now.Add(60 * time.Second)
				}
			} else {
				room.Status = RoomOpen
				room.CountdownEnds = time.Time{}
			}
		} else if room.Status != RoomCountdown {
			room.Status = RoomCountdown
			room.CountdownEnds = now.Add(60 * time.Second)
		}
		world.Rooms[room.ID] = room
		return nil
	})
}

func roomHasReadyPlayers(room MissionRoom) bool {
	for _, side := range room.Seats {
		for _, seat := range side {
			if seat.CallSign != "" && seat.Ready {
				return true
			}
		}
	}
	return false
}

// StartExpiredBattles advances mission rooms whose documented 60-second ready
// countdown has elapsed. Empty opposite seats become AI opponents for solo play.
func (s *Store) StartExpiredBattles(now time.Time) ([]MissionRoom, error) {
	var started []MissionRoom
	err := s.update(func(world *World) error {
		for id, room := range world.Rooms {
			if room.Status != RoomCountdown || room.CountdownEnds.After(now) {
				continue
			}
			var readyBySide [2][]Seat
			var unready []Seat
			for side := range room.Seats {
				for _, seat := range room.Seats[side] {
					if seat.CallSign == "" {
						continue
					}
					if seat.Ready || seat.AI {
						readyBySide[side] = append(readyBySide[side], seat)
					} else {
						unready = append(unready, seat)
					}
				}
			}
			readyCount := len(readyBySide[0]) + len(readyBySide[1])
			if readyCount == 0 {
				room.Status = RoomOpen
				room.CountdownEnds = time.Time{}
				for side := range room.Seats {
					for seatIndex := range room.Seats[side] {
						room.Seats[side][seatIndex].Ready = false
					}
				}
				world.Rooms[id] = room
				continue
			}
			for _, seat := range unready {
				if pilot, ok := world.Pilots[pilotKey(seat.CallSign)]; ok {
					pilot.Location.RoomID = 0
					pilot.Location.Side = 0
					pilot.Location.Seat = 0
					world.Pilots[pilotKey(seat.CallSign)] = pilot
				}
			}
			room.Seats = [2][LanceSeatCount]Seat{}
			if len(readyBySide[0]) > 0 && len(readyBySide[1]) > 0 {
				for side := range readyBySide {
					copy(room.Seats[side][:], readyBySide[side])
				}
			} else if readyCount == 1 {
				side := 0
				if len(readyBySide[1]) == 1 {
					side = 1
				}
				room.Seats[side][0] = readyBySide[side][0]
				room.Seats[1-side][0] = Seat{CallSign: "AI Opponent", Ready: true, AI: true}
			} else {
				readySeats := append(readyBySide[0], readyBySide[1]...)
				for index, seat := range readySeats {
					side := index % 2
					seatIndex := index / 2
					room.Seats[side][seatIndex] = seat
				}
			}
			room.Status = RoomInCombat
			room.CountdownEnds = time.Time{}
			for side := range room.Seats {
				for seatIndex, seat := range room.Seats[side] {
					if seat.CallSign == "" || seat.AI {
						continue
					}
					if pilot, ok := world.Pilots[pilotKey(seat.CallSign)]; ok {
						pilot.Location.RoomID = room.ID
						pilot.Location.Side = side
						pilot.Location.Seat = seatIndex
						world.Pilots[pilotKey(seat.CallSign)] = pilot
					}
				}
			}
			world.Rooms[id] = room
			if room.PlanetID == SolarisPlanetID {
				world.ensureSolarisRoomSlots(room.CityID, room.CadetOnly)
			}
			started = append(started, room)
		}
		return nil
	})
	sort.Slice(started, func(i, j int) bool { return started[i].ID < started[j].ID })
	return started, err
}

func (s *Store) ApplyBattleResult(result MatchResult) error {
	if strings.TrimSpace(result.MatchID) == "" {
		return errors.New("battle result needs a match ID")
	}
	if result.WinningSide < 0 || result.WinningSide > 1 {
		return errors.New("winning side must be lance alpha (0) or lance beta (1)")
	}
	if result.AttackingSide < 0 || result.AttackingSide > 1 {
		return errors.New("attacking side must be lance alpha (0) or lance beta (1)")
	}
	return s.update(func(world *World) error {
		if world.AppliedMatches[result.MatchID] {
			return nil
		}
		room, ok := world.Rooms[result.RoomID]
		if !ok || room.Status != RoomInCombat {
			return errors.New("battle result does not match an active room")
		}
		participants := result.ParticipantCallSigns
		if len(participants) == 0 {
			for side := range room.Seats {
				for _, seat := range room.Seats[side] {
					if seat.CallSign != "" && !seat.AI {
						participants = append(participants, seat.CallSign)
					}
				}
			}
		}
		seatSides := make(map[string]int)
		for side := range room.Seats {
			for _, seat := range room.Seats[side] {
				if seat.CallSign != "" && !seat.AI {
					seatSides[pilotKey(seat.CallSign)] = side
				}
			}
		}
		planet := world.Planets[room.PlanetID]
		for _, callSign := range participants {
			key := pilotKey(callSign)
			side, seated := seatSides[key]
			if !seated {
				continue
			}
			pilot, exists := world.Pilots[key]
			if !exists {
				continue
			}
			pilot.Battles++
			if side == result.WinningSide {
				pilot.Wins++
			}
			if room.PlanetID == SolarisPlanetID {
				// The guide confirms cadet rank progression and that graduated
				// Solaris return trips give no campaign rank or C-bills. Point
				// values are prototype defaults pending the original scoring table.
				if !pilot.Graduated {
					points := PrototypeLossPoints
					if side == result.WinningSide {
						points = PrototypeWinPoints
					}
					pilot.RankPoints += points
					if pilot.RankPoints >= GraduationRankPoints {
						pilot.RankPoints = GraduationRankPoints
						pilot.Graduated = true
						pilot.SolarisGraduationMedal = true
					}
				}
			} else {
				pilot.Credits += 50
			}
			world.Pilots[key] = pilot
		}
		if room.PlanetID != SolarisPlanetID && result.AttackingHouse != "" {
			shiftControl(&planet, result.AttackingHouse, result.WinningSide == result.AttackingSide)
			world.Planets[planet.ID] = planet
		}
		room.Status = RoomComplete
		world.Rooms[room.ID] = room
		if room.PlanetID == SolarisPlanetID {
			world.ensureSolarisRoomSlots(room.CityID, room.CadetOnly)
		}
		world.AppliedMatches[result.MatchID] = true
		return nil
	})
}

func (s *Store) update(change func(*World) error) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	previous := cloneWorld(s.world)
	if err := change(&s.world); err != nil {
		s.world = previous
		return err
	}
	if err := s.saveLocked(); err != nil {
		s.world = previous
		return err
	}
	return nil
}

func (s *Store) saveLocked() error {
	data, err := json.MarshalIndent(s.world, "", "  ")
	if err != nil {
		return fmt.Errorf("encode world state: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0700); err != nil {
		return fmt.Errorf("create world state directory: %w", err)
	}
	temporary := s.path + ".tmp"
	if err := os.WriteFile(temporary, append(data, '\n'), 0600); err != nil {
		return fmt.Errorf("write world state: %w", err)
	}
	if err := os.Rename(temporary, s.path); err != nil {
		_ = os.Remove(temporary)
		return fmt.Errorf("replace world state: %w", err)
	}
	return nil
}

func cloneWorld(world World) World {
	data, _ := json.Marshal(world)
	var clone World
	_ = json.Unmarshal(data, &clone)
	return clone
}

func isAdjacentToHouse(world World, planet Planet, house HouseID) bool {
	for _, neighborID := range planet.NeighborPlanetIDs {
		if neighbor, ok := world.Planets[neighborID]; ok && neighbor.Owner == house {
			return true
		}
	}
	return false
}

func knownHouse(world World, houseID HouseID) bool {
	for _, house := range world.Houses {
		if house.ID == houseID {
			return true
		}
	}
	return false
}

func solarisCity(world World, cityID int) (CityLobby, bool) {
	for _, city := range world.Planets[SolarisPlanetID].Cities {
		if city.ID == cityID {
			return city, true
		}
	}
	return CityLobby{}, false
}

func cityMechCapacity(city CityLobby) int {
	if city.MaxMechs > 0 {
		return city.MaxMechs
	}
	return 64
}

func cityMechOccupancy(world World, cityID int, excludedCallSign string) int {
	occupied := 0
	for _, pilot := range world.Pilots {
		if pilot.Location.PlanetID != SolarisPlanetID || pilot.Location.CityID != cityID || strings.EqualFold(pilot.CallSign, excludedCallSign) {
			continue
		}
		occupied += len(pilot.OwnedMechs)
	}
	return occupied
}

func availableSolarisCity(world World, preferredCityID int, excludedCallSign string) (CityLobby, error) {
	planet, ok := world.Planets[SolarisPlanetID]
	if !ok || len(planet.Cities) == 0 {
		return CityLobby{}, errors.New("Solaris has no city lobbies")
	}
	if preferredCityID != 0 {
		if preferred, ok := solarisCity(world, preferredCityID); ok && cityMechOccupancy(world, preferredCityID, excludedCallSign)+pilotMechCount(world, excludedCallSign) <= cityMechCapacity(preferred) {
			return preferred, nil
		}
	}
	mechs := pilotMechCount(world, excludedCallSign)
	for _, city := range planet.Cities {
		occupied := cityMechOccupancy(world, city.ID, excludedCallSign)
		if occupied+mechs <= cityMechCapacity(city) {
			return city, nil
		}
	}
	return CityLobby{}, errors.New("all Solaris cities are full")
}

func pilotMechCount(world World, callSign string) int {
	if pilot, ok := world.Pilots[pilotKey(callSign)]; ok && len(pilot.OwnedMechs) > 0 {
		return len(pilot.OwnedMechs)
	}
	return 1
}

func removePilotFromRooms(world *World, callSign string) {
	for id, room := range world.Rooms {
		changed := false
		for side := range room.Seats {
			for seatIndex, seat := range room.Seats[side] {
				if strings.EqualFold(seat.CallSign, callSign) {
					room.Seats[side][seatIndex] = Seat{}
					changed = true
				}
			}
		}
		if changed && room.Status == RoomCountdown && !roomHasReadyPlayers(room) {
			room.Status = RoomOpen
			room.CountdownEnds = time.Time{}
		}
		world.Rooms[id] = room
	}
}

func shiftControl(planet *Planet, attackingHouse HouseID, attackerWon bool) {
	if attackingHouse == planet.Owner {
		return
	}
	if _, ok := planet.ControlPercent[attackingHouse]; !ok {
		return
	}
	if attackerWon {
		if planet.Owner != "" {
			planet.ControlPercent[planet.Owner] -= PrototypeControlShift
			if planet.ControlPercent[planet.Owner] < 0 {
				planet.ControlPercent[planet.Owner] = 0
			}
		}
		planet.ControlPercent[attackingHouse] += PrototypeControlShift
		if planet.ControlPercent[attackingHouse] >= 50 {
			planet.Owner = attackingHouse
		}
		return
	}
	planet.ControlPercent[attackingHouse] -= PrototypeControlShift
	if planet.ControlPercent[attackingHouse] < 0 {
		planet.ControlPercent[attackingHouse] = 0
	}
	if planet.Owner != "" {
		planet.ControlPercent[planet.Owner] += PrototypeControlShift
	}
}
