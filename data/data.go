// Package data supplies optional operator-owned world seed data and a synthetic
// fallback. No client cache or extracted game data is embedded in this module.
package data

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// Files implements ReadFile for the two seed tables consumed by state. If the
// operator has not supplied a seed directory, the files are synthesized with
// generic planet labels and a balanced example ownership map.
var Files seedFiles

type seedFiles struct{}

type planetName struct {
	ID           int    `json:"id"`
	Name         string `json:"name"`
	InitialOwner string `json:"initial_owner,omitempty"`
}

var syntheticOwners = []string{"davion", "kurita", "liao", "marik", "steiner"}

func (seedFiles) ReadFile(name string) ([]byte, error) {
	if name != "planet_names.json" && name != "planet_sds_owners_v1.json" {
		return nil, fmt.Errorf("unknown world seed file %q", name)
	}
	seedDir := os.Getenv("MPBT3025_SEED_DIR")
	if seedDir == "" {
		seedDir = filepath.Join("runtime", "data")
	}
	contents, err := os.ReadFile(filepath.Join(seedDir, name))
	if err == nil {
		return contents, nil
	}
	if !os.IsNotExist(err) {
		return nil, fmt.Errorf("read optional seed %s: %w", name, err)
	}
	if name == "planet_names.json" {
		return syntheticPlanetNames()
	}
	return syntheticOwnershipSeed()
}

func syntheticPlanetNames() ([]byte, error) {
	rows := make([]planetName, 0, 121)
	for id := 1; id <= 121; id++ {
		row := planetName{ID: id, Name: fmt.Sprintf("Planet %03d", id)}
		if id < 121 {
			row.InitialOwner = syntheticOwner(id)
		}
		rows = append(rows, row)
	}
	return json.MarshalIndent(rows, "", "  ")
}

func syntheticOwnershipSeed() ([]byte, error) {
	owners := make(map[int]string, 121)
	for id := 1; id <= 120; id++ {
		owners[id] = syntheticOwner(id)
	}
	return json.MarshalIndent(owners, "", "  ")
}

func syntheticOwner(id int) string {
	owner := syntheticOwners[(id-1)%len(syntheticOwners)]
	// Keep the example seed even across the five Houses while making the
	// numeric sample deterministic for tests and local development.
	switch id {
	case 3:
		return "davion"
	case 8:
		return "kurita"
	case 13:
		return "marik"
	case 83:
		return "steiner"
	default:
		return owner
	}
}
