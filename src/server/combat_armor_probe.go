package server

import (
	"log"

	"mpbt3025/src/protocol/game"
)

// armorProbeForShot is a diagnostic, called under h.mu after room/fire
// reference validation. It trusts the collision report. Exactly one non-lethal
// stock medium-laser armor change per target avoids unsupported depletion,
// internal/critical damage and repeated-hit piloting rules. Never enable it as
// a substitute for authoritative combat. Mission teardown resets its budget.
func (h *sessionHub) armorProbeForShot(roomID int, shooter byte, hit protocol.CombatFireSelection, clientFire []byte) (protocol.PDU, bool) {
	if !h.experimentalCombatArmorProbe || hit.WeaponSlot != 0 || hit.TargetEntity < 0 || hit.TargetEntity == int(shooter) || hit.HitPart < 0 || hit.HitPart >= protocol.CombatArmorRegionCount {
		return protocol.PDU{}, false
	}
	target := byte(hit.TargetEntity)
	if h.combatArmorProbed[roomID][target] {
		return protocol.PDU{}, false
	}
	const laserDamage = 5           // Init(CWeapon,type3): table 0x690b99b4+3*140.
	const initialPilotingTarget = 4 // CMechObject constructor 0x69041026.
	armor := protocol.CombatCommandoArmor()[hit.HitPart]
	if armor <= laserDamage {
		return protocol.PDU{}, false
	}
	command, err := protocol.CombatDamageCommand(protocol.CombatDamageUpdate{
		EntityID: target, Slot: byte(21 + hit.HitPart), Remaining: armor - laserDamage,
		CreditEntity: shooter, PilotingTarget: initialPilotingTarget,
	})
	if err != nil {
		return protocol.PDU{}, false
	}
	fire, err := protocol.CombatPeerFire(shooter, clientFire)
	if err != nil {
		return protocol.PDU{}, false
	}
	// One stream preserves queue-before-apply ordering on both clients. The
	// local shooter's fire visitor skips its effect but still applies the queue.
	command = append(command, fire...)
	if h.combatArmorProbed == nil {
		h.combatArmorProbed = map[int]map[byte]bool{}
	}
	if h.combatArmorProbed[roomID] == nil {
		h.combatArmorProbed[roomID] = map[byte]bool{}
	}
	h.combatArmorProbed[roomID][target] = true
	log.Printf("DIAGNOSTIC armor probe mission %d: entity %d region %d armor %d -> %d, shooter %d; trusts collision report", roomID, target, hit.HitPart, armor, armor-laserDamage, shooter)
	return protocol.CombatData(command), true
}
