package query_test

import (
	"path/filepath"
	"testing"

	"github.com/CFMVCarlos/mhn-database/pkg/database"
	"github.com/CFMVCarlos/mhn-database/pkg/query"
)

func TestLookups(t *testing.T) {
	dbPath := filepath.Join("..", "..", "data", "mhn.db")
	db, err := database.OpenDB(dbPath)
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}
	defer db.Close()

	// 1. Monster Lookup
	m, err := query.LookupMonster(db, "Rathalos")
	if err != nil {
		t.Fatalf("monster lookup failed: %v", err)
	}
	if m.Name != "Rathalos" {
		t.Errorf("expected Rathalos, got %s", m.Name)
	}
	if len(m.Weaknesses) == 0 {
		t.Errorf("expected weaknesses for Rathalos")
	}
	if len(m.Materials) == 0 {
		t.Errorf("expected materials for Rathalos")
	}

	// 2. Weapon Lookup
	w, err := query.LookupWeapon(db, "Wyvern Blade", 0, 0)
	if err != nil {
		t.Fatalf("weapon lookup failed: %v", err)
	}
	if len(w.Levels) == 0 {
		t.Errorf("expected levels for Wyvern Blade")
	}

	// 3. Armor Lookup
	a, err := query.LookupArmor(db, "Rathalos Mail", 0, 0)
	if err != nil {
		t.Fatalf("armor lookup failed: %v", err)
	}
	if len(a.Levels) == 0 {
		t.Errorf("expected levels for Rathalos Mail")
	}

	// 4. Skill Lookup
	s, err := query.LookupSkill(db, "Critical Eye")
	if err != nil {
		t.Fatalf("skill lookup failed: %v", err)
	}
	if len(s.Levels) == 0 {
		t.Errorf("expected levels for Critical Eye")
	}
	if len(s.ArmorPieces) == 0 {
		t.Errorf("expected armor pieces with Critical Eye")
	}

	// 5. Material Lookup
	mat, err := query.LookupMaterial(db, "Wyvern Gem Shard")
	if err != nil {
		t.Fatalf("material lookup failed: %v", err)
	}
	if len(mat.Monsters) == 0 {
		t.Errorf("expected monsters dropping Wyvern Gem Shard")
	}

	// 6. Gear by Skill
	gear, err := query.FindGearBySkill(db, "Lock On", 1)
	if err != nil {
		t.Fatalf("find gear by skill failed: %v", err)
	}
	if len(gear) == 0 {
		t.Errorf("expected gear with Lock On")
	}

	// 7. Read-only SQL Runner
	sqlRes, err := query.RunSQL(db, "SELECT count(*) as cnt FROM monsters")
	if err != nil {
		t.Fatalf("sql query failed: %v", err)
	}
	if sqlRes["count"].(int) != 1 {
		t.Errorf("expected 1 row result from count query")
	}
}
