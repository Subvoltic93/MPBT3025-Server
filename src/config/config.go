// Package config defines the local emulator's command-line configuration.
package config

import (
	"errors"
	"flag"
)

type Config struct {
	Listen                                    string
	TraceDir                                  string
	WorldPath                                 string
	PlanetVCR                                 string
	PlanetVCRSolaris                          string
	PlanetStateSchemaProbe                    bool
	PlanetStateFullProbe                      bool
	PlanetStateIDMirrorProbe                  bool
	ExperimentalEmptySolarisRoomListAfterJoin bool
	ExperimentalPlanetRoomType                int
	ExperimentalCombatInit                    bool
	ExperimentalCombatArmorProbe              bool
	ExperimentalCombatDamage                  bool
	PlanetRedirect                            bool
	WorldListen                               string
}

func ParseFlags() Config {
	var cfg Config
	flag.StringVar(&cfg.Listen, "listen", "127.0.0.1:15000", "local Aries listener")
	flag.StringVar(&cfg.TraceDir, "trace-dir", "runtime/traces", "directory for local packet traces")
	flag.StringVar(&cfg.WorldPath, "world", "runtime/world.json", "persistent prototype campaign state")
	flag.StringVar(&cfg.PlanetVCR, "planet-vcr", "", "unsafe experimental VCR-derived planet state; prior live delivery terminated the client")
	flag.StringVar(&cfg.PlanetVCRSolaris, "planet-vcr-solaris", "", "unsafe experimental single-Solaris VCR record; standalone live delivery terminated the client")
	flag.BoolVar(&cfg.PlanetStateSchemaProbe, "planet-state-schema-probe", false, "send one numeric 0xa8 entry keyed 121 on the first global-state poll; state fields are unconfirmed")
	flag.BoolVar(&cfg.PlanetStateFullProbe, "planet-state-full-probe", false, "send 121 numeric 0xa8 entries keyed 1 through 121 on the first global-state poll")
	flag.BoolVar(&cfg.PlanetStateIDMirrorProbe, "planet-state-id-mirror-probe", false, "experimental: mirror IDs 1-121 into both 0xa8 scalar fields")
	flag.BoolVar(&cfg.ExperimentalEmptySolarisRoomListAfterJoin, "experimental-solaris-empty-room-list-after-join", false, "send a verified empty 0x97 room list after Solaris occupant presence; experiment only")
	flag.IntVar(&cfg.ExperimentalPlanetRoomType, "experimental-planet-room-type", -1, "send recovered 0x3e room update after Solaris join with lobby type 1 or 2; -1 disables")
	flag.BoolVar(&cfg.ExperimentalCombatInit, "experimental-combat-init", false, "send native-fixture-verified Title/COM-4D/Ready command setup after entering combat")
	flag.BoolVar(&cfg.ExperimentalCombatArmorProbe, "experimental-combat-armor-probe", false, "diagnostic: one non-lethal medium-laser armor update per target; requires experimental-combat-init and trusts reported collisions")
	flag.BoolVar(&cfg.ExperimentalCombatDamage, "experimental-combat-damage", false, "experimental cumulative COM-4D medium-laser damage with recharge, range and reported-pose envelope checks; requires experimental-combat-init")
	flag.BoolVar(&cfg.PlanetRedirect, "planet-redirect", false, "experimentally redirect successful planet joins to a second Aries listener")
	flag.StringVar(&cfg.WorldListen, "world-listen", "127.0.0.1:15001", "secondary Aries listener used with -planet-redirect")
	flag.Parse()
	return cfg
}

func (c Config) Validate() error {
	if c.ExperimentalCombatDamage && !c.ExperimentalCombatInit {
		return errors.New("experimental-combat-damage requires experimental-combat-init")
	}
	if c.ExperimentalCombatDamage && c.ExperimentalCombatArmorProbe {
		return errors.New("choose experimental-combat-damage or experimental-combat-armor-probe")
	}
	if c.ExperimentalCombatArmorProbe && !c.ExperimentalCombatInit {
		return errors.New("experimental-combat-armor-probe requires experimental-combat-init")
	}
	if c.ExperimentalPlanetRoomType != -1 && c.ExperimentalPlanetRoomType != 1 && c.ExperimentalPlanetRoomType != 2 {
		return errors.New("experimental-planet-room-type must be -1, 1, or 2")
	}
	return nil
}
