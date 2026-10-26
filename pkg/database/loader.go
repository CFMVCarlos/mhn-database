package database

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

var weaponCategoryNames = map[string]string{
	"SWORD_SHIELD":  "Sword & Shield",
	"DUAL_BLADES":   "Dual Blades",
	"GREAT_SWORD":   "Great Sword",
	"LONG_SWORD":    "Long Sword",
	"HAMMER":        "Hammer",
	"HUNTING_HORN":  "Hunting Horn",
	"LANCE":         "Lance",
	"GUNLANCE":      "Gunlance",
	"SWITCH_AXE":    "Switch Axe",
	"CHARGE_BLADE":  "Charge Blade",
	"INSECT_GLAIVE": "Insect Glaive",
	"LIGHT_BOWGUN":  "Light Bowgun",
	"HEAVY_BOWGUN":  "Heavy Bowgun",
	"BOW":           "Bow",
}

var armorCategoryNames = map[string]string{
	"HEAD":  "Head",
	"CHEST": "Chest",
	"ARMS":  "Arms",
	"WAIST": "Waist",
	"LEGS":  "Legs",
}

// BuildDatabase populates the SQLite database using raw JSON snapshots.
func BuildDatabase(rawDir, dbPath string) error {
	db, err := OpenDB(dbPath)
	if err != nil {
		return fmt.Errorf("open db: %w", err)
	}
	defer db.Close()

	// Load raw files
	monstersBytes, err := os.ReadFile(filepath.Join(rawDir, "monsters.json"))
	if err != nil {
		return fmt.Errorf("read monsters.json: %w", err)
	}
	skillsBytes, err := os.ReadFile(filepath.Join(rawDir, "skills.json"))
	if err != nil {
		return fmt.Errorf("read skills.json: %w", err)
	}
	weaponsBytes, err := os.ReadFile(filepath.Join(rawDir, "weapons.json"))
	if err != nil {
		return fmt.Errorf("read weapons.json: %w", err)
	}
	armorBytes, err := os.ReadFile(filepath.Join(rawDir, "armor.json"))
	if err != nil {
		return fmt.Errorf("read armor.json: %w", err)
	}

	var monsterDetails map[string]struct {
		Description string            `json:"description"`
		UsefulInfo  string            `json:"useful_info"`
		Trans       map[string]string `json:"translations"`
	}
	if detBytes, err := os.ReadFile(filepath.Join(rawDir, "monster_details.json")); err == nil {
		_ = json.Unmarshal(detBytes, &monsterDetails)
	}

	var materialsMap map[string]string
	if matBytes, err := os.ReadFile(filepath.Join(rawDir, "materials.json")); err == nil {
		_ = json.Unmarshal(matBytes, &materialsMap)
	}
	if materialsMap == nil {
		materialsMap = make(map[string]string)
	}

	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback()

	// 1. Process Skills
	if err := loadSkills(tx, skillsBytes); err != nil {
		return fmt.Errorf("load skills: %w", err)
	}

	// 2. Process Monsters
	if err := loadMonsters(tx, monstersBytes, monsterDetails, materialsMap); err != nil {
		return fmt.Errorf("load monsters: %w", err)
	}

	// 3. Process Weapons
	if err := loadWeapons(tx, weaponsBytes); err != nil {
		return fmt.Errorf("load weapons: %w", err)
	}

	// 4. Process Armor
	if err := loadArmor(tx, armorBytes); err != nil {
		return fmt.Errorf("load armor: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}

	fmt.Printf("✓ Successfully built SQLite database at %s\n", dbPath)
	return nil
}

func loadSkills(tx *sql.Tx, data []byte) error {
	var payload struct {
		GuideTranslations map[string]string `json:"guideTranslations"`
		Skills            map[string]struct {
			SkillID  int    `json:"skillId"`
			Kind     string `json:"kind"`
			Category string `json:"category"`
			MaxLevel int    `json:"maxLevel"`
			Levels   []struct {
				Level           int   `json:"level"`
				Description     string `json:"description"`
				ConditionAmount []int  `json:"conditionAmount"`
				EffectAmount    []int  `json:"effectAmount"`
			} `json:"levels"`
		} `json:"skills"`
		Driftstones map[string]struct {
			Id     string `json:"id"`
			Name   string `json:"name"`
			Skills []struct {
				Kind string `json:"kind"`
			} `json:"skills"`
		} `json:"driftstones"`
	}

	if err := json.Unmarshal(data, &payload); err != nil {
		return fmt.Errorf("unmarshal skills: %w", err)
	}

	stmtSkill, err := tx.Prepare(`INSERT OR REPLACE INTO skills (id, name, category, max_level) VALUES (?, ?, ?, ?)`)
	if err != nil {
		return err
	}
	defer stmtSkill.Close()

	stmtLevel, err := tx.Prepare(`INSERT OR REPLACE INTO skill_levels (skill_id, level, description) VALUES (?, ?, ?)`)
	if err != nil {
		return err
	}
	defer stmtLevel.Close()

	for k, s := range payload.Skills {
		name := payload.GuideTranslations[k]
		if name == "" {
			name = formatIdentifier(k)
		}
		if _, err := stmtSkill.Exec(k, name, s.Category, s.MaxLevel); err != nil {
			return err
		}

		for _, lvl := range s.Levels {
			descTpl := payload.GuideTranslations[lvl.Description]
			if descTpl == "" {
				descTpl = lvl.Description
			}
			desc := interpolateSkillDesc(descTpl, lvl.EffectAmount, lvl.ConditionAmount)
			if _, err := stmtLevel.Exec(k, lvl.Level, desc); err != nil {
				return err
			}
		}
	}

	stmtDrift, err := tx.Prepare(`INSERT OR REPLACE INTO driftstones (id, name, skills_json) VALUES (?, ?, ?)`)
	if err != nil {
		return err
	}
	defer stmtDrift.Close()

	for k, d := range payload.Driftstones {
		name := payload.GuideTranslations[d.Name]
		if name == "" {
			name = formatIdentifier(k)
		}
		var skillList []string
		for _, sk := range d.Skills {
			skillList = append(skillList, sk.Kind)
		}
		skillsJSON, _ := json.Marshal(skillList)
		if _, err := stmtDrift.Exec(k, name, string(skillsJSON)); err != nil {
			return err
		}
	}

	return nil
}

func interpolateSkillDesc(tpl string, effects, conditions []int) string {
	res := tpl
	for i, val := range effects {
		res = strings.ReplaceAll(res, fmt.Sprintf("{effect_amount_%d}", i+1), strconv.Itoa(val))
	}
	for i, val := range conditions {
		res = strings.ReplaceAll(res, fmt.Sprintf("{condition_amount_%d}", i+1), strconv.Itoa(val))
	}
	return res
}

func loadMonsters(tx *sql.Tx, data []byte, details map[string]struct {
	Description string            `json:"description"`
	UsefulInfo  string            `json:"useful_info"`
	Trans       map[string]string `json:"translations"`
}, materialsMap map[string]string) error {
	var payload struct {
		GuideTranslations map[string]string `json:"guideTranslations"`
		Monsters          []struct {
			ID             string   `json:"id"`
			Name           string   `json:"name"`
			Slug           string   `json:"slug"`
			Species        string   `json:"species"`
			Weakness       []string `json:"weakness"`
			Habitat        []string `json:"habitat"`
			Element        []string `json:"element"`
			ReleaseDate    string   `json:"releaseDate"`
			FieldIconURL   string   `json:"fieldIconUrl"`
			CoverImage     struct {
				Src string `json:"src"`
			} `json:"coverImage"`
			BreakableParts []struct {
				Name      string `json:"name"`
				BreakType string `json:"breakType"`
			} `json:"breakableParts"`
			ItemData []struct {
				Name       string `json:"name"`
				ItemRarity int    `json:"itemRarity"`
				MinGrade   int    `json:"minGrade"`
				ImageURL   string `json:"imageUrl"`
			} `json:"itemData"`
		} `json:"monsters"`
	}

	if err := json.Unmarshal(data, &payload); err != nil {
		return fmt.Errorf("unmarshal monsters: %w", err)
	}

	stmtMonster, err := tx.Prepare(`INSERT OR REPLACE INTO monsters 
		(id, name, slug, species, description, useful_info, habitats, elements, release_date, field_icon_url, cover_image_url) 
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return err
	}
	defer stmtMonster.Close()

	stmtWeakness, err := tx.Prepare(`INSERT OR REPLACE INTO monster_weaknesses (monster_id, element) VALUES (?, ?)`)
	if err != nil {
		return err
	}
	defer stmtWeakness.Close()

	stmtPart, err := tx.Prepare(`INSERT OR REPLACE INTO monster_breakable_parts (monster_id, part_name, break_type) VALUES (?, ?, ?)`)
	if err != nil {
		return err
	}
	defer stmtPart.Close()

	stmtMat, err := tx.Prepare(`INSERT OR REPLACE INTO monster_materials (monster_id, material_key, name, rarity, min_grade, image_url) VALUES (?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return err
	}
	defer stmtMat.Close()

	for _, m := range payload.Monsters {
		var desc, usefulInfo string
		if det, ok := details[m.Slug]; ok {
			desc = det.Description
			usefulInfo = det.UsefulInfo
		}

		var habitats []string
		for _, h := range m.Habitat {
			t := payload.GuideTranslations[h]
			if t != "" {
				habitats = append(habitats, t)
			} else {
				habitats = append(habitats, formatIdentifier(h))
			}
		}

		var elements []string
		for _, el := range m.Element {
			t := payload.GuideTranslations[el]
			if t != "" {
				elements = append(elements, t)
			} else {
				elements = append(elements, formatIdentifier(el))
			}
		}

		species := payload.GuideTranslations[m.Species]
		if species == "" {
			species = formatIdentifier(m.Species)
		}

		if _, err := stmtMonster.Exec(
			m.ID, m.Name, m.Slug, species, desc, usefulInfo,
			strings.Join(habitats, ", "), strings.Join(elements, ", "),
			m.ReleaseDate, m.FieldIconURL, m.CoverImage.Src,
		); err != nil {
			return err
		}

		for _, w := range m.Weakness {
			elem := strings.Title(strings.ToLower(w))
			if _, err := stmtWeakness.Exec(m.ID, elem); err != nil {
				return err
			}
		}

		for _, bp := range m.BreakableParts {
			partName := formatIdentifier(bp.Name)
			if _, err := stmtPart.Exec(m.ID, partName, bp.BreakType); err != nil {
				return err
			}
		}

		for _, item := range m.ItemData {
			matName := materialsMap[item.Name]
			if matName == "" {
				matName = payload.GuideTranslations[item.Name]
			}
			if matName == "" {
				matName = formatIdentifier(item.Name)
			}
			if _, err := stmtMat.Exec(m.ID, item.Name, matName, item.ItemRarity, item.MinGrade, item.ImageURL); err != nil {
				return err
			}
		}
	}

	return nil
}

func loadWeapons(tx *sql.Tx, data []byte) error {
	var payload struct {
		GuideTranslations map[string]string `json:"guideTranslations"`
		Weapons           map[string]struct {
			ID       string `json:"id"`
			Category string `json:"category"`
			Element  string `json:"element"`
			Series   string `json:"series"`
			Grades   []struct {
				Grade   int    `json:"grade"`
				Name    string `json:"name"`
				Skills  []struct {
					Kind  string `json:"kind"`
					Level int    `json:"level"`
				} `json:"skills"`
				Levels []struct {
					Level         int `json:"level"`
					Attack        int `json:"attack"`
					ElementAttack int `json:"elementAttack"`
					Critical      int `json:"critical"`
				} `json:"levels"`
			} `json:"grades"`
		} `json:"weapons"`
	}

	if err := json.Unmarshal(data, &payload); err != nil {
		return fmt.Errorf("unmarshal weapons: %w", err)
	}

	stmtWeapon, err := tx.Prepare(`INSERT OR REPLACE INTO weapons 
		(id, name, category, category_name, element, series, series_name, min_grade, max_grade) 
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return err
	}
	defer stmtWeapon.Close()

	stmtWLevel, err := tx.Prepare(`INSERT OR REPLACE INTO weapon_levels 
		(weapon_id, grade, level, name, attack, element_attack, affinity, skills_json) 
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return err
	}
	defer stmtWLevel.Close()

	stmtWSkill, err := tx.Prepare(`INSERT OR REPLACE INTO weapon_skills 
		(weapon_id, skill_id, unlock_grade, skill_level) VALUES (?, ?, ?, ?)`)
	if err != nil {
		return err
	}
	defer stmtWSkill.Close()

	for k, w := range payload.Weapons {
		catName := weaponCategoryNames[w.Category]
		if catName == "" {
			catName = formatIdentifier(w.Category)
		}
		elemName := formatIdentifier(w.Element)
		if strings.HasPrefix(strings.ToUpper(elemName), "NO") {
			elemName = "None"
		}

		minGrade := 100
		maxGrade := 0
		var baseName string

		for _, g := range w.Grades {
			if g.Grade < minGrade {
				minGrade = g.Grade
			}
			if g.Grade > maxGrade {
				maxGrade = g.Grade
			}
			gName := payload.GuideTranslations[g.Name]
			if gName == "" {
				gName = g.Name
			}
			if baseName == "" {
				baseName = stripTrailingDigits(gName)
			}
		}

		if baseName == "" {
			baseName = formatIdentifier(k)
		}
		if minGrade == 100 {
			minGrade = 1
		}

		seriesName := formatIdentifier(w.Series)

		if _, err := stmtWeapon.Exec(k, baseName, w.Category, catName, elemName, w.Series, seriesName, minGrade, maxGrade); err != nil {
			return err
		}

		for _, g := range w.Grades {
			gName := payload.GuideTranslations[g.Name]
			if gName == "" {
				gName = fmt.Sprintf("%s Grade %d", baseName, g.Grade)
			}

			skillsJSON, _ := json.Marshal(g.Skills)
			for _, sk := range g.Skills {
				if _, err := stmtWSkill.Exec(k, sk.Kind, g.Grade, sk.Level); err != nil {
					return err
				}
			}

			for _, lvl := range g.Levels {
				lvlName := fmt.Sprintf("%s Lv %d", gName, lvl.Level)
				if _, err := stmtWLevel.Exec(k, g.Grade, lvl.Level, lvlName, lvl.Attack, lvl.ElementAttack, lvl.Critical, string(skillsJSON)); err != nil {
					return err
				}
			}
		}
	}

	return nil
}

func loadArmor(tx *sql.Tx, data []byte) error {
	var payload struct {
		GuideTranslations map[string]string `json:"guideTranslations"`
		Armor             map[string]struct {
			ID       string `json:"id"`
			Category string `json:"category"`
			Series   string `json:"series"`
			Grades   []struct {
				Grade           int    `json:"grade"`
				Name            string `json:"name"`
				DriftsmeltSlots int    `json:"driftsmeltSlots"`
				Defense         []int  `json:"defense"`
				Skills          []struct {
					Kind  string `json:"kind"`
					Level int    `json:"level"`
				} `json:"skills"`
			} `json:"grades"`
		} `json:"armor"`
	}

	if err := json.Unmarshal(data, &payload); err != nil {
		return fmt.Errorf("unmarshal armor: %w", err)
	}

	stmtArmor, err := tx.Prepare(`INSERT OR REPLACE INTO armor_pieces 
		(id, name, category, category_name, series, series_name, min_grade, max_grade) 
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return err
	}
	defer stmtArmor.Close()

	stmtALevel, err := tx.Prepare(`INSERT OR REPLACE INTO armor_levels 
		(armor_id, grade, level, name, defense, driftsmelt_slots, skills_json) 
		VALUES (?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return err
	}
	defer stmtALevel.Close()

	stmtASkill, err := tx.Prepare(`INSERT OR REPLACE INTO armor_skills 
		(armor_id, skill_id, unlock_grade, skill_level) VALUES (?, ?, ?, ?)`)
	if err != nil {
		return err
	}
	defer stmtASkill.Close()

	for k, a := range payload.Armor {
		catName := armorCategoryNames[a.Category]
		if catName == "" {
			catName = formatIdentifier(a.Category)
		}

		minGrade := 100
		maxGrade := 0
		var baseName string

		for _, g := range a.Grades {
			if g.Grade < minGrade {
				minGrade = g.Grade
			}
			if g.Grade > maxGrade {
				maxGrade = g.Grade
			}
			gName := payload.GuideTranslations[g.Name]
			if gName == "" {
				gName = g.Name
			}
			if baseName == "" {
				baseName = stripTrailingDigits(gName)
			}
		}

		if baseName == "" {
			baseName = formatIdentifier(k)
		}
		if minGrade == 100 {
			minGrade = 1
		}

		seriesName := formatIdentifier(a.Series)

		if _, err := stmtArmor.Exec(k, baseName, a.Category, catName, a.Series, seriesName, minGrade, maxGrade); err != nil {
			return err
		}

		for _, g := range a.Grades {
			gName := payload.GuideTranslations[g.Name]
			if gName == "" {
				gName = fmt.Sprintf("%s Grade %d", baseName, g.Grade)
			}

			skillsJSON, _ := json.Marshal(g.Skills)
			for _, sk := range g.Skills {
				if _, err := stmtASkill.Exec(k, sk.Kind, g.Grade, sk.Level); err != nil {
					return err
				}
			}

			for lvlIdx, defVal := range g.Defense {
				lvl := lvlIdx + 1
				lvlName := fmt.Sprintf("%s Lv %d", gName, lvl)
				if _, err := stmtALevel.Exec(k, g.Grade, lvl, lvlName, defVal, g.DriftsmeltSlots, string(skillsJSON)); err != nil {
					return err
				}
			}
		}
	}

	return nil
}

func formatIdentifier(s string) string {
	s = strings.TrimPrefix(s, "AREA_TYPE_")
	s = strings.TrimPrefix(s, "SERIES_")
	s = strings.TrimPrefix(s, "ELEMENT_")
	s = strings.ReplaceAll(s, "_", " ")
	return strings.Title(strings.ToLower(s))
}

func stripTrailingDigits(s string) string {
	re := regexp.MustCompile(`\s+\d+$`)
	return strings.TrimSpace(re.ReplaceAllString(s, ""))
}
