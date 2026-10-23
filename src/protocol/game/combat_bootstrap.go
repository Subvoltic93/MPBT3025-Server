package protocol

import "fmt"

// CombatPilot describes a starter COM-4D in the experimental native setup.
// Combat entity IDs and teams use the combat DLL's own numbering; these are
// separate from the shell's one-based seats and Alpha/Beta headings.
type CombatPilot struct {
	ID, Team           byte
	PlayerID, CallSign string
	X, Y               int32
}

// CombatBootstrap assembles the native Title and stock COM-4D commands.
// Environment defaults and the COM-4D loadout are recovered from the DLL's
// FakeStart/FakeACat routines. This is a controlled setup trial, not combat
// simulation: movement and damage processing still require a server loop.
// Ready must follow the client's preparation acknowledgement, once its HUD
// nodes exist. Sending it in this same stream crashes CHUDBogie::AddBogie.
func CombatBootstrap(localID byte, players []CombatPilot) (PDU, error) {
	if len(players) == 0 || len(players) > 8 || int(localID) >= len(players) {
		return PDU{}, fmt.Errorf("invalid native combat roster or local entity %d", localID)
	}
	w := combatWriter{}
	// Title uses Desert (0), generated terrain 0, Earth gravity, normal
	// friction/pressure, no heat penalty. The four engine bytes reproduce
	// FakeStart setup (0x6905c692), adjusting the exact Mech count. AddNewMech
	// at 0x690576d7 waits for this count before advancing from state 1 to 2.
	w.title("Solaris", localID, 0, [4]uint32{0, 980, 100, 100}, 0, [4]byte{byte(len(players)), 2, 0, 5})
	for index, pilot := range players {
		if int(pilot.ID) != index || pilot.Team > 1 {
			return PDU{}, fmt.Errorf("invalid native combat entity %d/team %d", pilot.ID, pilot.Team)
		}
		w.commando(pilot)
	}
	if w.err != nil {
		return PDU{}, w.err
	}
	return CombatData(w.data), nil
}

type combatWriter struct {
	data []byte
	err  error
}

func (w *combatWriter) number(value uint32, digits int) {
	limit := uint64(1)
	for range digits {
		limit *= 85
	}
	if uint64(value) >= limit {
		w.err = fmt.Errorf("native combat value %d exceeds %d base85 digits", value, digits)
		return
	}
	start := len(w.data)
	w.data = append(w.data, make([]byte, digits)...)
	for i := digits - 1; i >= 0; i-- {
		w.data[start+i] = byte(value%85) + '!'
		value /= 85
	}
}

func (w *combatWriter) text(value string) {
	for _, b := range []byte(value) {
		if b < ' ' || b > '~' {
			w.err = fmt.Errorf("native combat text must be printable ASCII")
			return
		}
	}
	w.number(uint32(len(value)), 1)
	w.data = append(w.data, value...)
}

func (w *combatWriter) coordinate(value int32) {
	const bias = 26100312 // 85^4/2; decoder 0x69074cc0 subtracts this.
	if value < -bias || value > bias {
		w.err = fmt.Errorf("native combat coordinate %d is out of range", value)
		return
	}
	w.number(uint32(int64(value)+bias), 4)
}

// Fields follow the serialization subobject at TitleCommand+4, writer
// 0x69072be0. Engine bytes correspond to whole-object offsets 0x55..0x58;
// their complete semantic names are still being recovered.
func (w *combatWriter) title(name string, localID, climate byte, values [4]uint32, heat byte, engine [4]byte) {
	w.data = append(w.data, 'i')
	w.text(name)
	w.number(uint32(localID), 1)
	w.number(uint32(climate), 1)
	for _, value := range values {
		w.number(value, 3)
	}
	w.number(uint32(heat), 2)
	for _, b := range engine[:3] {
		w.number(uint32(b), 1)
	}
	w.number(uint32(engine[3]), 2)
}

func (w *combatWriter) commando(pilot CombatPilot) {
	w.data = append(w.data, 'a')
	w.number(uint32(pilot.ID), 1)
	w.number(uint32(pilot.Team), 1)
	w.text(pilot.PlayerID)
	w.text(pilot.CallSign)
	w.text("")
	w.coordinate(pilot.X)
	w.coordinate(pilot.Y)
	w.number(0, 1) // heading
	w.text("COM-4D")
	w.text("Commando")
	// Native writer 0x6906e8be reads the type byte (0x40) here, despite
	// decoder 0x6906ebf8 storing this field at whole-object offset 0x90.
	w.number(0x40, 3)
	w.number(6, 3) // stock walking speed
	w.number(0, 4)
	w.number(0, 4)
	w.number(5, 1)
	w.number(30, 2)
	w.number(12, 1)
	w.number(0, 1)
	// Stock armor, weapon, component, and ammunition records copied from
	// FakeACat's MechID8 branch and shared component initialization. The
	// first list's wire order is location/value, unlike its in-memory pair.
	armor := [...]byte{8, 8, 10, 10, 15, 13, 13, 11, 8, 8, 27}
	w.number(uint32(len(armor)), 1)
	for location, value := range armor {
		w.number(uint32(location), 1)
		w.number(uint32(value), 1)
	}
	weapons := [...][3]byte{{3, 1, 0}, {0, 0, 0}, {12, 4, 0}}
	w.number(uint32(len(weapons)), 1)
	for _, record := range weapons {
		for _, value := range record {
			w.number(uint32(value), 1)
		}
	}
	components := make([][2]byte, 0, 30)
	for index := byte(0); index < 21; index++ {
		components = append(components, [2]byte{index, 0})
	}
	components = append(components, [][2]byte{{24, 5}, {24, 5}, {24, 6}, {24, 6}, {22, 2}, {22, 2}, {22, 3}, {22, 3}, {22, 7}}...)
	w.number(uint32(len(components)), 1)
	for _, record := range components {
		w.number(uint32(record[0]), 1)
		w.number(uint32(record[1]), 1)
	}
	w.number(1, 1)
	w.number(11, 1)
	w.number(6, 1)
}
