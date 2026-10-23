// Package state owns campaign rules and persistent game state. It has no wire
// protocol code so rules can be changed without rewriting packet codecs.
package state

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"mpbt3025/data"
)

const (
	SolarisPlanetID             = 121
	GraduationRankPoints        = 1200
	LanceSeatCount              = 4
	SolarisCityCount            = 16
	SolarisRoomPerCity          = 3
	PrototypeWinPoints          = 100
	PrototypeLossPoints         = 40
	PrototypeControlShift       = 10
	initialOwnershipSeedVersion = 2
)

type HouseID string

const (
	HouseDavion  HouseID = "davion"
	HouseKurita  HouseID = "kurita"
	HouseLiao    HouseID = "liao"
	HouseMarik   HouseID = "marik"
	HouseSteiner HouseID = "steiner"
)

var houseOrder = []HouseID{HouseDavion, HouseKurita, HouseLiao, HouseMarik, HouseSteiner}

type House struct {
	ID   HouseID `json:"id"`
	Name string  `json:"name"`
}

type PlanetType string

const (
	CapitalPlanet    PlanetType = "capital"
	DivisionHQPlanet PlanetType = "division_hq"
	BrigadeHQPlanet  PlanetType = "brigade_hq"
	RegimentPlanet   PlanetType = "regiment_hq"
	TrainingPlanet   PlanetType = "training"
)

type Planet struct {
	ID                 int               `json:"id"`
	Name               string            `json:"name"`
	Type               PlanetType        `json:"type"`
	Owner              HouseID           `json:"owner,omitempty"`
	ControlPercent     map[HouseID]int   `json:"control_percent"`
	EconomyValue       int               `json:"economy_value"`
	MaxMechWeightClass int               `json:"max_mech_weight_class"`
	NeighborPlanetIDs  []int             `json:"neighbor_planet_ids"`
	Sectors            []StrategicSector `json:"sectors,omitempty"`
	Cities             []CityLobby       `json:"cities,omitempty"`
}

type StrategicSector struct {
	ID        int    `json:"id"`
	Name      string `json:"name"`
	MaxPilots int    `json:"max_pilots"`
}

type CityLobby struct {
	ID       int    `json:"id"`
	Name     string `json:"name"`
	MaxMechs int    `json:"max_mechs"`
}

// UnmarshalJSON accepts the prototype's earlier max_pilots spelling while
// writing the manual's actual capacity unit, 'Mechs, on the next save.
func (c *CityLobby) UnmarshalJSON(data []byte) error {
	var stored struct {
		ID        int    `json:"id"`
		Name      string `json:"name"`
		MaxMechs  int    `json:"max_mechs"`
		MaxPilots int    `json:"max_pilots"`
	}
	if err := json.Unmarshal(data, &stored); err != nil {
		return err
	}
	c.ID = stored.ID
	c.Name = stored.Name
	c.MaxMechs = stored.MaxMechs
	if c.MaxMechs == 0 {
		c.MaxMechs = stored.MaxPilots
	}
	return nil
}

type Pilot struct {
	CallSign   string `json:"call_sign"`
	PlayerID   uint32 `json:"player_id"`
	RankPoints int    `json:"rank_points"`
	Graduated  bool   `json:"graduated"`
	// SolarisGraduationMedal is awarded with graduation at 1,200 rank points.
	// Graduated is retained as the compatibility/policy flag used by older saves.
	SolarisGraduationMedal  bool      `json:"solaris_graduation_medal,omitempty"`
	SolarisCityPickerOrigin *Location `json:"solaris_city_picker_origin,omitempty"`
	House                   HouseID   `json:"house,omitempty"`
	Credits                 int       `json:"credits"`
	OwnedMechs              []string  `json:"owned_mechs"`
	Location                Location  `json:"location"`
	Battles                 int       `json:"battles"`
	Wins                    int       `json:"wins"`
	CreatedAt               time.Time `json:"created_at"`
}

type Location struct {
	PlanetID int `json:"planet_id"`
	CityID   int `json:"city_id,omitempty"`
	SectorID int `json:"sector_id,omitempty"`
	RoomID   int `json:"room_id,omitempty"`
	Side     int `json:"side,omitempty"`
	Seat     int `json:"seat,omitempty"`
}

type RoomStatus string

const (
	RoomOpen      RoomStatus = "open"
	RoomCountdown RoomStatus = "countdown"
	RoomInCombat  RoomStatus = "in_combat"
	RoomComplete  RoomStatus = "complete"
)

type Seat struct {
	MechID   uint32 `json:"native_mech_id,omitempty"`
	CallSign string `json:"call_sign,omitempty"`
	Ready    bool   `json:"ready"`
	AI       bool   `json:"ai,omitempty"`
}

type MissionRoom struct {
	ID            int                     `json:"id"`
	PlanetID      int                     `json:"planet_id"`
	CityID        int                     `json:"city_id"`
	SectorID      int                     `json:"sector_id,omitempty"`
	Name          string                  `json:"name"`
	Mission       string                  `json:"mission"`
	CadetOnly     bool                    `json:"cadet_only,omitempty"`
	Status        RoomStatus              `json:"status"`
	Seats         [2][LanceSeatCount]Seat `json:"seats"`
	CountdownEnds time.Time               `json:"countdown_ends,omitempty"`
}

type World struct {
	SchemaVersion        int                 `json:"schema_version"`
	OwnershipSeedVersion int                 `json:"ownership_seed_version,omitempty"`
	Houses               []House             `json:"houses"`
	Planets              map[int]Planet      `json:"planets"`
	Pilots               map[string]Pilot    `json:"pilots"`
	Rooms                map[int]MissionRoom `json:"rooms"`
	AppliedMatches       map[string]bool     `json:"applied_matches"`
	SalaryClaims         map[string]bool     `json:"salary_claims"`
	NextPlayerID         uint32              `json:"next_player_id"`
	NextRoomID           int                 `json:"next_room_id"`
}

type PlanetName struct {
	ID           int     `json:"id"`
	Name         string  `json:"name"`
	InitialOwner HouseID `json:"initial_owner,omitempty"`
}

// DefaultWorld creates an editable prototype campaign. Planet names come from
// the client's VCR cache, including its PlanetInfo House field. Control starts
// at 100% for the listed owner. Solaris has the cache's special House ID 7
// and remains neutral; adjacency and mission setup are still synthetic.
func DefaultWorld() (World, error) {
	data, err := data.Files.ReadFile("planet_names.json")
	if err != nil {
		return World{}, fmt.Errorf("read embedded planet names: %w", err)
	}
	var names []PlanetName
	if err := json.Unmarshal(data, &names); err != nil {
		return World{}, fmt.Errorf("decode embedded planet names: %w", err)
	}

	world := World{
		SchemaVersion:        1,
		OwnershipSeedVersion: initialOwnershipSeedVersion,
		Houses:               []House{{HouseDavion, "Davion"}, {HouseKurita, "Kurita"}, {HouseLiao, "Liao"}, {HouseMarik, "Marik"}, {HouseSteiner, "Steiner"}},
		Planets:              make(map[int]Planet, len(names)),
		Pilots:               make(map[string]Pilot),
		Rooms:                make(map[int]MissionRoom),
		AppliedMatches:       make(map[string]bool),
		SalaryClaims:         make(map[string]bool),
		NextPlayerID:         1,
		NextRoomID:           1,
	}
	for _, entry := range names {
		planet := prototypePlanet(entry)
		world.Planets[planet.ID] = planet
	}
	world.seedSolarisRooms()
	world.seedCampaignRooms()
	return world, nil
}

func prototypePlanet(entry PlanetName) Planet {
	planet := Planet{
		ID:                 entry.ID,
		Name:               entry.Name,
		Type:               RegimentPlanet,
		ControlPercent:     make(map[HouseID]int, len(houseOrder)),
		EconomyValue:       1 + (entry.ID % 5),
		MaxMechWeightClass: 2,
	}
	for _, house := range houseOrder {
		planet.ControlPercent[house] = 0
	}
	if entry.ID == SolarisPlanetID {
		planet.Type = TrainingPlanet
		planet.EconomyValue = 0
		planet.MaxMechWeightClass = 4
		for cityID := 1; cityID <= SolarisCityCount; cityID++ {
			planet.Cities = append(planet.Cities, CityLobby{
				ID:       cityID,
				Name:     fmt.Sprintf("Solaris City %02d", cityID),
				MaxMechs: 64,
			})
		}
		return planet
	}

	// Initial affiliations come from the preserved client's PlanetInfo table.
	// Its House field covers every non-Solaris planet.
	if entry.InitialOwner != "" {
		planet.Owner = entry.InitialOwner
		planet.ControlPercent[entry.InitialOwner] = 100
	}
	if entry.ID%25 == 1 {
		planet.Type = CapitalPlanet
		planet.MaxMechWeightClass = 4
	} else if entry.ID%13 == 0 {
		planet.Type = DivisionHQPlanet
		planet.MaxMechWeightClass = 3
	} else if entry.ID%7 == 0 {
		planet.Type = BrigadeHQPlanet
		planet.MaxMechWeightClass = 2
	}
	left := entry.ID - 1
	if left == 0 {
		left = 120
	}
	right := entry.ID + 1
	if right > 120 {
		right = 1
	}
	planet.NeighborPlanetIDs = []int{left, right}
	for sectorID, name := range []string{"North Approach", "Central Front", "South Approach"} {
		planet.Sectors = append(planet.Sectors, StrategicSector{
			ID:        sectorID + 1,
			Name:      name,
			MaxPilots: 64,
		})
	}
	return planet
}

// migrateLegacySyntheticOwnership upgrades untouched prototype or SDS-seeded
// ownership to the complete PlanetInfo cache seed. Campaign and operator
// changes survive because each older seed must match exactly before migration.
func (w *World) migrateLegacySyntheticOwnership() (bool, error) {
	if w.OwnershipSeedVersion >= initialOwnershipSeedVersion {
		return false, nil
	}
	var untouched bool
	switch w.OwnershipSeedVersion {
	case 0:
		untouched = w.hasLegacySyntheticOwnership()
	case 1:
		var err error
		untouched, err = w.hasSDSSeedOwnership()
		if err != nil {
			return false, err
		}
	}
	if !untouched {
		return false, nil
	}

	seed, err := DefaultWorld()
	if err != nil {
		return false, err
	}
	for planetID, planet := range w.Planets {
		seeded := seed.Planets[planetID]
		planet.Owner = seeded.Owner
		planet.ControlPercent = make(map[HouseID]int, len(seeded.ControlPercent))
		for house, percent := range seeded.ControlPercent {
			planet.ControlPercent[house] = percent
		}
		w.Planets[planetID] = planet
	}
	w.OwnershipSeedVersion = initialOwnershipSeedVersion
	return true, nil
}

func (w World) hasSDSSeedOwnership() (bool, error) {
	data, err := data.Files.ReadFile("planet_sds_owners_v1.json")
	if err != nil {
		return false, err
	}
	var owners map[int]HouseID
	if err := json.Unmarshal(data, &owners); err != nil {
		return false, err
	}
	if len(w.Planets) != 121 || len(w.AppliedMatches) != 0 {
		return false, nil
	}
	for _, pilot := range w.Pilots {
		if pilot.Battles != 0 || pilot.Wins != 0 {
			return false, nil
		}
	}
	for planetID := 1; planetID <= SolarisPlanetID; planetID++ {
		planet, ok := w.Planets[planetID]
		if !ok || len(planet.ControlPercent) != len(houseOrder) || planet.Owner != owners[planetID] {
			return false, nil
		}
		for _, house := range houseOrder {
			want := 0
			if house == planet.Owner && house != "" {
				want = 100
			}
			if planet.ControlPercent[house] != want {
				return false, nil
			}
		}
	}
	return true, nil
}

func (w World) hasLegacySyntheticOwnership() bool {
	if len(w.Planets) != 121 || len(w.AppliedMatches) != 0 {
		return false
	}
	for _, pilot := range w.Pilots {
		if pilot.Battles != 0 || pilot.Wins != 0 {
			return false
		}
	}
	for planetID := 1; planetID <= 121; planetID++ {
		planet, ok := w.Planets[planetID]
		if !ok || len(planet.ControlPercent) != len(houseOrder) {
			return false
		}
		var expectedOwner HouseID
		if planetID != SolarisPlanetID {
			expectedOwner = houseOrder[(planetID-1)%len(houseOrder)]
		}
		if planet.Owner != expectedOwner {
			return false
		}
		for _, house := range houseOrder {
			expectedPercent := 0
			if house == expectedOwner {
				expectedPercent = 100
			}
			if planet.ControlPercent[house] != expectedPercent {
				return false
			}
		}
	}
	return true
}

func (w *World) seedSolarisRooms() {
	solaris := w.Planets[SolarisPlanetID]
	for _, city := range solaris.Cities {
		w.ensureSolarisRoomSlots(city.ID, true)
		w.ensureSolarisRoomSlots(city.ID, false)
	}
}

// ensureSolarisRoomSlots maintains a small set of empty missions per city and
// pilot tier. In-combat and completed rooms remain in history but stop taking
// the place of an empty practice mission in the Mission List.
func (w *World) ensureSolarisRoomSlots(cityID int, cadetOnly bool) bool {
	if w.Rooms == nil {
		w.Rooms = make(map[int]MissionRoom)
	}
	changed := false
	if w.NextRoomID < 1 {
		w.NextRoomID = 1
		changed = true
	}
	openCount := 0
	nameIndex := 0
	for id, room := range w.Rooms {
		if room.PlanetID != SolarisPlanetID || room.CityID != cityID || room.CadetOnly != cadetOnly {
			continue
		}
		nameIndex++
		if room.Status == RoomOpen && roomHasNoPilots(room) {
			openCount++
		}
		if id >= w.NextRoomID {
			w.NextRoomID = id + 1
			changed = true
		}
	}
	for openCount < SolarisRoomPerCity {
		nameIndex++
		room := MissionRoom{
			ID:        w.NextRoomID,
			PlanetID:  SolarisPlanetID,
			CityID:    cityID,
			Name:      fmt.Sprintf("Solaris Mission %d", nameIndex),
			Mission:   "Solaris Practice",
			CadetOnly: cadetOnly,
			Status:    RoomOpen,
		}
		if cadetOnly {
			room.Name = fmt.Sprintf("Cadet Mission %d", nameIndex)
			room.Mission = "Cadet Practice"
		}
		w.Rooms[room.ID] = room
		w.NextRoomID++
		openCount++
		changed = true
	}
	return changed
}

func roomHasNoPilots(room MissionRoom) bool {
	for _, side := range room.Seats {
		for _, seat := range side {
			if seat.CallSign != "" {
				return false
			}
		}
	}
	return true
}

func (w *World) seedCampaignRooms() {
	for planetID := 1; planetID < SolarisPlanetID; planetID++ {
		planet := w.Planets[planetID]
		for _, sector := range planet.Sectors {
			room := MissionRoom{
				ID:       w.NextRoomID,
				PlanetID: planet.ID,
				SectorID: sector.ID,
				Name:     "Frontline Skirmish",
				Mission:  "Territory Assault",
				Status:   RoomOpen,
			}
			w.Rooms[room.ID] = room
			w.NextRoomID++
		}
	}
}

func (w World) Validate() error {
	if w.SchemaVersion != 1 {
		return fmt.Errorf("unsupported world schema version %d", w.SchemaVersion)
	}
	if len(w.Planets) != 121 {
		return fmt.Errorf("world has %d planets, want 121", len(w.Planets))
	}
	if _, ok := w.Planets[SolarisPlanetID]; !ok {
		return errors.New("world has no Solaris planet")
	}
	playerIDs := make(map[uint32]string, len(w.Pilots))
	var maxPlayerID uint32
	for key, pilot := range w.Pilots {
		if key != pilotKey(pilot.CallSign) {
			return fmt.Errorf("pilot key %q does not match call sign %q", key, pilot.CallSign)
		}
		if pilot.PlayerID == 0 {
			return fmt.Errorf("pilot %q has no player ID", pilot.CallSign)
		}
		if previous, exists := playerIDs[pilot.PlayerID]; exists {
			return fmt.Errorf("pilots %q and %q share player ID %d", previous, pilot.CallSign, pilot.PlayerID)
		}
		playerIDs[pilot.PlayerID] = pilot.CallSign
		if pilot.PlayerID > maxPlayerID {
			maxPlayerID = pilot.PlayerID
		}
	}
	if w.NextPlayerID == 0 || w.NextPlayerID <= maxPlayerID {
		return fmt.Errorf("next player ID %d must be greater than assigned ID %d", w.NextPlayerID, maxPlayerID)
	}
	return nil
}

func pilotKey(callSign string) string { return strings.ToLower(strings.TrimSpace(callSign)) }
