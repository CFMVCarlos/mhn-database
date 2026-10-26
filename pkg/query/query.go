package query

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
)

type MonsterResult struct {
	ID            string               `json:"id"`
	Name          string               `json:"name"`
	Slug          string               `json:"slug"`
	Species       string               `json:"species"`
	Description   string               `json:"description"`
	UsefulInfo    string               `json:"useful_info"`
	Habitats      string               `json:"habitats"`
	Elements      string               `json:"elements"`
	ReleaseDate   string               `json:"release_date"`
	Weaknesses    []string             `json:"weaknesses"`
	BreakableParts []BreakablePartResult `json:"breakable_parts"`
	Materials     []MaterialResult     `json:"materials"`
}

type BreakablePartResult struct {
	PartName  string `json:"part_name"`
	BreakType string `json:"break_type"`
}

type MaterialResult struct {
	Name     string `json:"name"`
	Rarity   int    `json:"rarity"`
	MinGrade int    `json:"min_grade"`
}

type WeaponResult struct {
	ID           string             `json:"id"`
	Name         string             `json:"name"`
	Category     string             `json:"category"`
	CategoryName string             `json:"category_name"`
	Element      string             `json:"element"`
	Series       string             `json:"series"`
	MinGrade     int                `json:"min_grade"`
	MaxGrade     int                `json:"max_grade"`
	Levels       []WeaponLevelResult `json:"levels,omitempty"`
	Skills       []EquipmentSkill   `json:"skills,omitempty"`
}

type WeaponLevelResult struct {
	Grade         int    `json:"grade"`
	Level         int    `json:"level"`
	Name          string `json:"name"`
	Attack        int    `json:"attack"`
	ElementAttack int    `json:"element_attack"`
	Affinity      int    `json:"affinity"`
	SkillsJSON    string `json:"skills_json"`
}

type ArmorResult struct {
	ID           string            `json:"id"`
	Name         string            `json:"name"`
	Category     string            `json:"category"`
	CategoryName string            `json:"category_name"`
	Series       string            `json:"series"`
	MinGrade     int               `json:"min_grade"`
	MaxGrade     int               `json:"max_grade"`
	Levels       []ArmorLevelResult `json:"levels,omitempty"`
	Skills       []EquipmentSkill  `json:"skills,omitempty"`
}

type ArmorLevelResult struct {
	Grade           int    `json:"grade"`
	Level           int    `json:"level"`
	Name            string `json:"name"`
	Defense         int    `json:"defense"`
	DriftsmeltSlots int    `json:"driftsmelt_slots"`
	SkillsJSON      string `json:"skills_json"`
}

type EquipmentSkill struct {
	SkillID     string `json:"skill_id"`
	SkillName   string `json:"skill_name"`
	UnlockGrade int    `json:"unlock_grade"`
	SkillLevel  int    `json:"skill_level"`
}

type SkillResult struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Category    string            `json:"category"`
	MaxLevel    int               `json:"max_level"`
	Levels      []SkillLevelResult `json:"levels"`
	ArmorPieces []GearWithSkill   `json:"armor_pieces"`
	Weapons     []GearWithSkill   `json:"weapons"`
}

type SkillLevelResult struct {
	Level       int    `json:"level"`
	Description string `json:"description"`
}

type GearWithSkill struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Category    string `json:"category"`
	UnlockGrade int    `json:"unlock_grade"`
	SkillLevel  int    `json:"skill_level"`
}

type MaterialSourceResult struct {
	MaterialName string         `json:"material_name"`
	Rarity       int            `json:"rarity"`
	Monsters     []MonsterDrop  `json:"monsters"`
}

type MonsterDrop struct {
	MonsterID   string `json:"monster_id"`
	MonsterName string `json:"monster_name"`
	Rarity      int    `json:"rarity"`
	MinGrade    int    `json:"min_grade"`
}

// LookupMonster finds monster details by ID, slug, or name.
func LookupMonster(db *sql.DB, query string) (*MonsterResult, error) {
	row := db.QueryRow(`
		SELECT id, name, slug, species, description, useful_info, habitats, elements, release_date
		FROM monsters
		WHERE id = ? OR slug = ? OR name LIKE ?
		ORDER BY CASE WHEN id = ? THEN 0 WHEN name = ? THEN 1 ELSE 2 END
		LIMIT 1
	`, query, query, "%"+query+"%", query, query)

	var m MonsterResult
	var desc, uInfo, species sql.NullString
	if err := row.Scan(&m.ID, &m.Name, &m.Slug, &species, &desc, &uInfo, &m.Habitats, &m.Elements, &m.ReleaseDate); err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("monster %q not found", query)
		}
		return nil, err
	}
	m.Species = species.String
	m.Description = desc.String
	m.UsefulInfo = uInfo.String

	// Weaknesses
	wRows, err := db.Query(`SELECT element FROM monster_weaknesses WHERE monster_id = ? ORDER BY element`, m.ID)
	if err == nil {
		defer wRows.Close()
		for wRows.Next() {
			var elem string
			if err := wRows.Scan(&elem); err == nil {
				m.Weaknesses = append(m.Weaknesses, elem)
			}
		}
	}

	// Breakable parts
	pRows, err := db.Query(`SELECT part_name, break_type FROM monster_breakable_parts WHERE monster_id = ? ORDER BY id`, m.ID)
	if err == nil {
		defer pRows.Close()
		for pRows.Next() {
			var p BreakablePartResult
			if err := pRows.Scan(&p.PartName, &p.BreakType); err == nil {
				m.BreakableParts = append(m.BreakableParts, p)
			}
		}
	}

	// Materials
	mRows, err := db.Query(`SELECT name, rarity, min_grade FROM monster_materials WHERE monster_id = ? ORDER BY rarity ASC, min_grade ASC`, m.ID)
	if err == nil {
		defer mRows.Close()
		for mRows.Next() {
			var mat MaterialResult
			if err := mRows.Scan(&mat.Name, &mat.Rarity, &mat.MinGrade); err == nil {
				m.Materials = append(m.Materials, mat)
			}
		}
	}

	return &m, nil
}

// LookupWeapon finds a weapon by ID or name with optional grade/level filtering.
func LookupWeapon(db *sql.DB, query string, grade, level int) (*WeaponResult, error) {
	row := db.QueryRow(`
		SELECT id, name, category, category_name, element, series, min_grade, max_grade
		FROM weapons
		WHERE id = ? OR name LIKE ?
		ORDER BY CASE WHEN id = ? THEN 0 WHEN name = ? THEN 1 ELSE 2 END
		LIMIT 1
	`, query, "%"+query+"%", query, query)

	var w WeaponResult
	if err := row.Scan(&w.ID, &w.Name, &w.Category, &w.CategoryName, &w.Element, &w.Series, &w.MinGrade, &w.MaxGrade); err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("weapon %q not found", query)
		}
		return nil, err
	}

	// Skills
	sRows, err := db.Query(`
		SELECT ws.skill_id, s.name, ws.unlock_grade, ws.skill_level
		FROM weapon_skills ws
		JOIN skills s ON ws.skill_id = s.id
		WHERE ws.weapon_id = ?
		ORDER BY ws.unlock_grade ASC
	`, w.ID)
	if err == nil {
		defer sRows.Close()
		for sRows.Next() {
			var sk EquipmentSkill
			if err := sRows.Scan(&sk.SkillID, &sk.SkillName, &sk.UnlockGrade, &sk.SkillLevel); err == nil {
				w.Skills = append(w.Skills, sk)
			}
		}
	}

	// Levels
	sqlStr := `SELECT grade, level, name, attack, element_attack, affinity, skills_json FROM weapon_levels WHERE weapon_id = ?`
	var args []interface{}
	args = append(args, w.ID)

	if grade > 0 {
		sqlStr += ` AND grade = ?`
		args = append(args, grade)
	}
	if level > 0 {
		sqlStr += ` AND level = ?`
		args = append(args, level)
	}
	sqlStr += ` ORDER BY grade ASC, level ASC`

	lRows, err := db.Query(sqlStr, args...)
	if err == nil {
		defer lRows.Close()
		for lRows.Next() {
			var lvl WeaponLevelResult
			var sJSON sql.NullString
			if err := lRows.Scan(&lvl.Grade, &lvl.Level, &lvl.Name, &lvl.Attack, &lvl.ElementAttack, &lvl.Affinity, &sJSON); err == nil {
				lvl.SkillsJSON = sJSON.String
				w.Levels = append(w.Levels, lvl)
			}
		}
	}

	return &w, nil
}

// LookupArmor finds an armor piece by ID or name with optional grade/level filtering.
func LookupArmor(db *sql.DB, query string, grade, level int) (*ArmorResult, error) {
	row := db.QueryRow(`
		SELECT id, name, category, category_name, series, min_grade, max_grade
		FROM armor_pieces
		WHERE id = ? OR name LIKE ?
		ORDER BY CASE WHEN id = ? THEN 0 WHEN name = ? THEN 1 ELSE 2 END
		LIMIT 1
	`, query, "%"+query+"%", query, query)

	var a ArmorResult
	if err := row.Scan(&a.ID, &a.Name, &a.Category, &a.CategoryName, &a.Series, &a.MinGrade, &a.MaxGrade); err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("armor %q not found", query)
		}
		return nil, err
	}

	// Skills
	sRows, err := db.Query(`
		SELECT ask.skill_id, s.name, ask.unlock_grade, ask.skill_level
		FROM armor_skills ask
		JOIN skills s ON ask.skill_id = s.id
		WHERE ask.armor_id = ?
		ORDER BY ask.unlock_grade ASC
	`, a.ID)
	if err == nil {
		defer sRows.Close()
		for sRows.Next() {
			var sk EquipmentSkill
			if err := sRows.Scan(&sk.SkillID, &sk.SkillName, &sk.UnlockGrade, &sk.SkillLevel); err == nil {
				a.Skills = append(a.Skills, sk)
			}
		}
	}

	// Levels
	sqlStr := `SELECT grade, level, name, defense, driftsmelt_slots, skills_json FROM armor_levels WHERE armor_id = ?`
	var args []interface{}
	args = append(args, a.ID)

	if grade > 0 {
		sqlStr += ` AND grade = ?`
		args = append(args, grade)
	}
	if level > 0 {
		sqlStr += ` AND level = ?`
		args = append(args, level)
	}
	sqlStr += ` ORDER BY grade ASC, level ASC`

	lRows, err := db.Query(sqlStr, args...)
	if err == nil {
		defer lRows.Close()
		for lRows.Next() {
			var lvl ArmorLevelResult
			var sJSON sql.NullString
			if err := lRows.Scan(&lvl.Grade, &lvl.Level, &lvl.Name, &lvl.Defense, &lvl.DriftsmeltSlots, &sJSON); err == nil {
				lvl.SkillsJSON = sJSON.String
				a.Levels = append(a.Levels, lvl)
			}
		}
	}

	return &a, nil
}

// LookupSkill finds a skill and returns its levels and all gear bestowing it.
func LookupSkill(db *sql.DB, query string) (*SkillResult, error) {
	row := db.QueryRow(`
		SELECT id, name, category, max_level
		FROM skills
		WHERE id = ? OR name LIKE ?
		ORDER BY CASE WHEN id = ? THEN 0 WHEN name = ? THEN 1 ELSE 2 END
		LIMIT 1
	`, query, "%"+query+"%", query, query)

	var s SkillResult
	var cat sql.NullString
	if err := row.Scan(&s.ID, &s.Name, &cat, &s.MaxLevel); err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("skill %q not found", query)
		}
		return nil, err
	}
	s.Category = cat.String

	// Skill levels
	lvlRows, err := db.Query(`SELECT level, description FROM skill_levels WHERE skill_id = ? ORDER BY level ASC`, s.ID)
	if err == nil {
		defer lvlRows.Close()
		for lvlRows.Next() {
			var sl SkillLevelResult
			if err := lvlRows.Scan(&sl.Level, &sl.Description); err == nil {
				s.Levels = append(s.Levels, sl)
			}
		}
	}

	// Armor pieces providing this skill
	aRows, err := db.Query(`
		SELECT a.id, a.name, a.category_name, ask.unlock_grade, ask.skill_level
		FROM armor_skills ask
		JOIN armor_pieces a ON ask.armor_id = a.id
		WHERE ask.skill_id = ?
		ORDER BY a.name ASC, ask.unlock_grade ASC
	`, s.ID)
	if err == nil {
		defer aRows.Close()
		for aRows.Next() {
			var g GearWithSkill
			if err := aRows.Scan(&g.ID, &g.Name, &g.Category, &g.UnlockGrade, &g.SkillLevel); err == nil {
				s.ArmorPieces = append(s.ArmorPieces, g)
			}
		}
	}

	// Weapons providing this skill
	wRows, err := db.Query(`
		SELECT w.id, w.name, w.category_name, ws.unlock_grade, ws.skill_level
		FROM weapon_skills ws
		JOIN weapons w ON ws.weapon_id = w.id
		WHERE ws.skill_id = ?
		ORDER BY w.name ASC, ws.unlock_grade ASC
	`, s.ID)
	if err == nil {
		defer wRows.Close()
		for wRows.Next() {
			var g GearWithSkill
			if err := wRows.Scan(&g.ID, &g.Name, &g.Category, &g.UnlockGrade, &g.SkillLevel); err == nil {
				s.Weapons = append(s.Weapons, g)
			}
		}
	}

	return &s, nil
}

// LookupMaterial finds which monsters drop a material.
func LookupMaterial(db *sql.DB, query string) (*MaterialSourceResult, error) {
	rows, err := db.Query(`
		SELECT mm.name, mm.rarity, mm.min_grade, m.id, m.name
		FROM monster_materials mm
		JOIN monsters m ON mm.monster_id = m.id
		WHERE mm.name LIKE ? OR mm.material_key = ?
		ORDER BY mm.rarity ASC, m.name ASC
	`, "%"+query+"%", query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var res MaterialSourceResult
	for rows.Next() {
		var matName string
		var rarity, minGrade int
		var mID, mName string
		if err := rows.Scan(&matName, &rarity, &minGrade, &mID, &mName); err != nil {
			continue
		}
		if res.MaterialName == "" {
			res.MaterialName = matName
			res.Rarity = rarity
		}
		res.Monsters = append(res.Monsters, MonsterDrop{
			MonsterID:   mID,
			MonsterName: mName,
			Rarity:      rarity,
			MinGrade:    minGrade,
		})
	}

	if len(res.Monsters) == 0 {
		return nil, fmt.Errorf("material %q not found", query)
	}

	return &res, nil
}

// FindGearBySkill searches for equipment bestowing a skill name with optional minimum skill level.
func FindGearBySkill(db *sql.DB, skillQuery string, minLevel int) ([]GearWithSkill, error) {
	if minLevel <= 0 {
		minLevel = 1
	}

	query := `
		SELECT a.id, a.name, a.category_name, ask.unlock_grade, ask.skill_level
		FROM armor_skills ask
		JOIN armor_pieces a ON ask.armor_id = a.id
		JOIN skills s ON ask.skill_id = s.id
		WHERE (s.name LIKE ? OR s.id = ?) AND ask.skill_level >= ?
		UNION ALL
		SELECT w.id, w.name, w.category_name, ws.unlock_grade, ws.skill_level
		FROM weapon_skills ws
		JOIN weapons w ON ws.weapon_id = w.id
		JOIN skills s ON ws.skill_id = s.id
		WHERE (s.name LIKE ? OR s.id = ?) AND ws.skill_level >= ?
		ORDER BY skill_level DESC, unlock_grade ASC
	`
	pattern := "%" + skillQuery + "%"
	rows, err := db.Query(query, pattern, skillQuery, minLevel, pattern, skillQuery, minLevel)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []GearWithSkill
	for rows.Next() {
		var g GearWithSkill
		if err := rows.Scan(&g.ID, &g.Name, &g.Category, &g.UnlockGrade, &g.SkillLevel); err == nil {
			results = append(results, g)
		}
	}

	return results, nil
}

// RunSQL runs a read-only SQL query and returns formatted column headers and string rows.
func RunSQL(db *sql.DB, sqlQuery string) (map[string]interface{}, error) {
	trimmed := strings.TrimSpace(sqlQuery)
	upper := strings.ToUpper(trimmed)
	if !strings.HasPrefix(upper, "SELECT") && !strings.HasPrefix(upper, "WITH") && !strings.HasPrefix(upper, "PRAGMA TABLE_INFO") {
		return nil, fmt.Errorf("only SELECT/read-only queries are permitted")
	}

	rows, err := db.Query(sqlQuery)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	cols, err := rows.Columns()
	if err != nil {
		return nil, err
	}

	var results []map[string]interface{}
	for rows.Next() {
		values := make([]interface{}, len(cols))
		valPtrs := make([]interface{}, len(cols))
		for i := range values {
			valPtrs[i] = &values[i]
		}

		if err := rows.Scan(valPtrs...); err != nil {
			return nil, err
		}

		rowMap := make(map[string]interface{})
		for i, col := range cols {
			val := values[i]
			if b, ok := val.([]byte); ok {
				rowMap[col] = string(b)
			} else {
				rowMap[col] = val
			}
		}
		results = append(results, rowMap)
	}

	return map[string]interface{}{
		"columns": cols,
		"count":   len(results),
		"rows":    results,
	}, nil
}

// ToJSON helper converts any struct to indented JSON string
func ToJSON(v interface{}) string {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Sprintf("{\"error\": %q}", err.Error())
	}
	return string(b)
}
