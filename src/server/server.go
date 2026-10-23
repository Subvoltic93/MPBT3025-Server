package server

import (
	"log"
	"net"
	"os"

	"mpbt3025/src/config"
	"mpbt3025/src/protocol/game"
	"mpbt3025/src/state"
)

// Run starts the local emulated server using the parsed command-line configuration.
func Run(cfg config.Config) {

	world, err := state.Open(cfg.WorldPath)
	if err != nil {
		log.Fatalf("Open campaign state: %v", err)
	}
	if err := world.ResetSessionPresence(); err != nil {
		log.Fatalf("Reset disconnected session presence: %v", err)
	}
	snapshot := world.Snapshot()
	log.Printf("Campaign state loaded: %d planets, %d mission rooms, %d pilots", len(snapshot.Planets), len(snapshot.Rooms), len(snapshot.Pilots))

	var planetStateReply *protocol.PDU
	selectedSeeds := 0
	for _, selected := range []bool{cfg.PlanetVCR != "", cfg.PlanetVCRSolaris != "", cfg.PlanetStateSchemaProbe, cfg.PlanetStateFullProbe, cfg.PlanetStateIDMirrorProbe} {
		if selected {
			selectedSeeds++
		}
	}
	if selectedSeeds > 1 {
		log.Fatal("choose only one planet-state source or probe")
	}
	if cfg.PlanetVCR != "" {
		seed, err := protocol.PlanetStateListFromVCR(cfg.PlanetVCR)
		if err != nil {
			log.Fatalf("Load planet-state seed: %v", err)
		}
		pduSize := len(protocol.Encode(seed))
		planetStateReply = &seed
		log.Printf("Loaded %d-byte experimental planet-state PDU from %s; oversized replies use inferred split-buffer framing (live client trial terminated)", pduSize, cfg.PlanetVCR)
	}
	if cfg.PlanetVCRSolaris != "" {
		seed, err := protocol.SolarisPlanetStateListFromVCR(cfg.PlanetVCRSolaris)
		if err != nil {
			log.Fatalf("Load Solaris planet-state seed: %v", err)
		}
		planetStateReply = &seed
		log.Printf("Loaded %d-byte experimental single-Solaris planet-state PDU from %s; its cache-record layout is unconfirmed", len(protocol.Encode(seed)), cfg.PlanetVCRSolaris)
	}
	if cfg.PlanetStateSchemaProbe {
		probe := protocol.SolarisPlanetStateSchemaProbe()
		planetStateReply = &probe
		log.Printf("Enabled one-entry 0xa8 schema probe (%d encoded bytes); record semantics are unconfirmed", len(protocol.Encode(probe)))
	}
	if cfg.PlanetStateFullProbe {
		probe := protocol.FullPlanetStateSchemaProbe()
		planetStateReply = &probe
		log.Printf("Enabled 121-entry 0xa8 schema probe (%d encoded bytes); record semantics are unconfirmed", len(protocol.Encode(probe)))
	}
	if cfg.PlanetStateIDMirrorProbe {
		probe := protocol.PlanetStateIDMirrorProbe()
		planetStateReply = &probe
		log.Printf("Enabled 121-entry 0xa8 ID mirror probe (%d encoded bytes); second scalar semantics are unconfirmed", len(protocol.Encode(probe)))
	}

	if err := os.MkdirAll(cfg.TraceDir, 0700); err != nil {
		log.Fatal(err)
	}
	listener, err := net.Listen("tcp", cfg.Listen)
	if err != nil {
		log.Fatal(err)
	}
	defer listener.Close()
	log.Printf("Local bootstrap listening on %s", listener.Addr())
	hub := newSessionHub(world)
	hub.planetRoomType = cfg.ExperimentalPlanetRoomType
	hub.experimentalCombatInit = cfg.ExperimentalCombatInit
	hub.experimentalCombatArmorProbe = cfg.ExperimentalCombatArmorProbe
	startMissionScheduler(world, hub)
	if cfg.PlanetRedirect {
		worldListener, err := net.Listen("tcp", cfg.WorldListen)
		if err != nil {
			log.Fatalf("Listen on secondary world address: %v", err)
		}
		defer worldListener.Close()
		log.Printf("Experimental secondary Aries listener on %s", worldListener.Addr())
		go serveConnections(worldListener, cfg.TraceDir, planetStateReply, world, "", hub, cfg.ExperimentalEmptySolarisRoomListAfterJoin)
	}
	redirectAddress := ""
	if cfg.PlanetRedirect {
		redirectAddress = cfg.WorldListen
	}
	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Fatal(err)
		}
		go func() {
			defer conn.Close()
			if err := serve(conn, cfg.TraceDir, planetStateReply, world, redirectAddress, hub, cfg.ExperimentalEmptySolarisRoomListAfterJoin); err != nil {
				log.Printf("%s: %v", conn.RemoteAddr(), err)
			}
		}()
	}
}

func planetStateOwnerSnapshot(world state.World, sequence uint32) protocol.PDU {
	entries := make([]protocol.PlanetStateWireEntry, 121)
	for i := range entries {
		id := i + 1
		entries[i].Key = uint32(id)
		switch world.Planets[id].Owner {
		case state.HouseDavion:
			entries[i].ValueB = 1
		case state.HouseKurita:
			entries[i].ValueB = 2
		case state.HouseLiao:
			entries[i].ValueB = 3
		case state.HouseMarik:
			entries[i].ValueB = 4
		case state.HouseSteiner:
			entries[i].ValueB = 5
		}
	}
	return protocol.PlanetStateListUpdate(entries, sequence, 0)
}
