package protocol

import "fmt"

const CombatArmorRegionCount = 11

// CombatDamageUpdate is an absolute native state assignment, not a damage
// delta. Slots 21..31 hold remaining armor and 32..39 remaining internal
// structure. Other slots address components/weapons and need separate rules.
// The credit entity is passed to the native damage-event callback. Piloting
// target is the final byte, independently identified by SetPilotingTarget.
// Encoding a report does not establish a valid hit or calculate those fields.
type CombatDamageUpdate struct {
	EntityID, Slot, Remaining, CreditEntity, PilotingTarget byte
}

// CombatDamageCommand matches native writer 0x6906baa0 (key 0x42, wire c).
// For the recovered laser path, queue assignments before the matching fire
// command, which requests application. This encoder is not wired into the relay.
func CombatDamageCommand(update CombatDamageUpdate) ([]byte, error) {
	fields := [...]byte{update.EntityID, update.Slot, update.Remaining, update.CreditEntity, update.PilotingTarget}
	command := []byte{'c'}
	for _, field := range fields {
		if field >= 85 {
			return nil, fmt.Errorf("native damage field %d exceeds one base85 digit", field)
		}
		command = append(command, field+'!')
	}
	return command, nil
}
