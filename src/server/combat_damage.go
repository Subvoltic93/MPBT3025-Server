package server

import (
	"log"
	"time"

	"mpbt3025/src/protocol/game"
)

const mediumLaserRecharge = 5 * time.Second // Native Init +0x28 and IsReady deadline.
const mediumLaserDamage = 5
const mediumLaserMaxRange = 27000 // Native GetMaxRange 270m, in hundredths.
const combatPoseMaxAge = 2 * time.Second

type combatMechState struct {
	Armor          [11]byte
	Internal       [8]byte
	WeaponDisabled [3]bool
	Destroyed      bool
	NextLaser      time.Time
}

type combatDamageRoom struct{ Mechs map[byte]combatMechState }

func newCombatDamageRoom(roster map[string]byte) *combatDamageRoom {
	r := &combatDamageRoom{Mechs: map[byte]combatMechState{}}
	for _, id := range roster {
		r.Mechs[id] = combatMechState{Armor: protocol.CombatCommandoArmor(),
			Internal: [8]byte{5, 5, 7, 7, 10, 7, 7, 9}}
	}
	return r
}

var armorInternal = [11]int{0, 1, 2, 3, 4, 5, 6, 4, 5, 6, 7}
var armorParent = [11]int{5, 6, 5, 6, -1, 4, 4, -1, 7, 7, -1}
var commandoWeaponInternal = [3]int{1, 0, 4}

// resolveCombatDamage mirrors native resolver 0x690500c0 for the initialized
// COM-4D. Outputs are absolute remaining-value assignments. Native differential
// fixtures cover every region, overflow, missing internals and weapon disable.
func resolveCombatDamage(mech *combatMechState, region, damage int, target, shooter byte) []protocol.CombatDamageUpdate {
	var updates []protocol.CombatDamageUpdate
	add := func(slot int, remaining byte) {
		updates = append(updates, protocol.CombatDamageUpdate{EntityID: target, Slot: byte(slot),
			Remaining: remaining, CreditEntity: shooter, PilotingTarget: 4})
	}
	for damage > 0 && region >= 0 {
		internal := armorInternal[region]
		if mech.Internal[internal] == 0 && armorParent[region] >= 0 {
			region = armorParent[region]
			continue
		}
		if armor := int(mech.Armor[region]); armor > 0 {
			remaining := max(armor-damage, 0)
			mech.Armor[region] = byte(remaining)
			add(21+region, byte(remaining))
			damage = max(damage-armor, 0)
		}
		if damage == 0 {
			break
		}
		before := int(mech.Internal[internal])
		remaining := max(before-damage, 0)
		mech.Internal[internal] = byte(remaining)
		add(32+internal, byte(remaining))
		if remaining == 0 {
			for weapon, location := range commandoWeaponInternal {
				if location == internal {
					mech.WeaponDisabled[weapon] = true
					add(40+weapon, 0)
				}
			}
		}
		damage = max(damage-before, 0)
		region = armorParent[region]
	}
	mech.Destroyed = mech.Internal[4] == 0 || mech.Internal[7] == 0
	return updates
}

func squaredDistance(a, b [3]int32) int64 {
	var squared int64
	for i := range a {
		d := int64(a[i]) - int64(b[i])
		squared += d * d
	}
	return squared
}

// plausibleLaserHit is an experimental envelope policy, not a recovered native
// ray/mesh collision test. Pose and collision reports remain client supplied.
func (h *sessionHub) plausibleLaserHit(roomID int, shooter, target byte, hit protocol.CombatFireSelection, now time.Time) bool {
	poses, times := h.combatPositionReports[roomID], h.combatPositionTimes[roomID]
	source, sourceOK := poses[shooter]
	destination, targetOK := poses[target]
	for _, id := range []byte{shooter, target} {
		age := now.Sub(times[id])
		if times[id].IsZero() || age < 0 || age > combatPoseMaxAge {
			return false
		}
	}
	if !sourceOK || !targetOK || !hit.HasHitPoint {
		return false
	}
	point, base := hit.HitPoint, destination.WorldPoint()
	dx, dz := int64(point[0])-int64(base[0]), int64(point[2])-int64(base[2])
	height := int64(point[1]) - int64(base[1])
	// Explicit trial limits: 10m horizontal radius, -1..20m above reported
	// terrain height. Mesh occlusion, aim and per-part shape are still unknown.
	if dx*dx+dz*dz > 1000*1000 || height < -100 || height > 2000 {
		return false
	}
	return squaredDistance(source.WorldPoint(), point) <= mediumLaserMaxRange*mediumLaserMaxRange
}

// damageForShot runs under h.mu after stream and authenticated-room checks.
// Every medium-laser report consumes recharge, including misses and rejected
// hit reports; duplicate reports within recharge cannot decrease armor again.
func (h *sessionHub) damageForShot(roomID int, shooter byte, hit protocol.CombatFireSelection, clientFire []byte, now time.Time) (protocol.PDU, bool) {
	if !h.experimentalCombatDamage || hit.WeaponSlot != 0 {
		return protocol.PDU{}, false
	}
	room := h.combatDamageRooms[roomID]
	if room == nil {
		return protocol.PDU{}, false
	}
	source, ok := room.Mechs[shooter]
	if !ok || source.Destroyed || source.WeaponDisabled[0] || now.Before(source.NextLaser) {
		return protocol.PDU{}, false
	}
	source.NextLaser = now.Add(mediumLaserRecharge)
	room.Mechs[shooter] = source
	if hit.TargetEntity < 0 || hit.TargetEntity == int(shooter) || hit.HitPart < 0 || hit.HitPart >= 11 {
		return protocol.PDU{}, false
	}
	target := byte(hit.TargetEntity)
	mech, ok := room.Mechs[target]
	if !ok || mech.Destroyed || !h.plausibleLaserHit(roomID, shooter, target, hit, now) {
		return protocol.PDU{}, false
	}
	updates := resolveCombatDamage(&mech, hit.HitPart, mediumLaserDamage, target, shooter)
	var commands []byte
	for _, update := range updates {
		command, err := protocol.CombatDamageCommand(update)
		if err != nil {
			return protocol.PDU{}, false
		}
		commands = append(commands, command...)
	}
	fire, err := protocol.CombatPeerFire(shooter, clientFire)
	if err != nil || len(commands) == 0 {
		return protocol.PDU{}, false
	}
	commands = append(commands, fire...)
	room.Mechs[target] = mech
	log.Printf("EXPERIMENTAL laser damage mission %d: shooter %d target %d region %d armor %v internal %v destroyed=%t; collision envelope policy", roomID, shooter, target, hit.HitPart, mech.Armor, mech.Internal, mech.Destroyed)
	return protocol.CombatData(commands), true
}
