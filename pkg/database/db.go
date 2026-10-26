package database

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

const SchemaSQL = `
PRAGMA foreign_keys = ON;
PRAGMA journal_mode = WAL;
PRAGMA synchronous = NORMAL;

CREATE TABLE IF NOT EXISTS monsters (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    slug TEXT NOT NULL UNIQUE,
    species TEXT,
    description TEXT,
    useful_info TEXT,
    habitats TEXT,
    elements TEXT,
    release_date TEXT,
    field_icon_url TEXT,
    cover_image_url TEXT
);

CREATE TABLE IF NOT EXISTS monster_weaknesses (
    monster_id TEXT NOT NULL,
    element TEXT NOT NULL,
    PRIMARY KEY (monster_id, element),
    FOREIGN KEY (monster_id) REFERENCES monsters(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS monster_breakable_parts (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    monster_id TEXT NOT NULL,
    part_name TEXT NOT NULL,
    break_type TEXT NOT NULL,
    FOREIGN KEY (monster_id) REFERENCES monsters(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS monster_materials (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    monster_id TEXT NOT NULL,
    material_key TEXT NOT NULL,
    name TEXT NOT NULL,
    rarity INTEGER NOT NULL,
    min_grade INTEGER,
    image_url TEXT,
    UNIQUE (monster_id, material_key),
    FOREIGN KEY (monster_id) REFERENCES monsters(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS skills (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    category TEXT,
    max_level INTEGER
);

CREATE TABLE IF NOT EXISTS skill_levels (
    skill_id TEXT NOT NULL,
    level INTEGER NOT NULL,
    description TEXT NOT NULL,
    PRIMARY KEY (skill_id, level),
    FOREIGN KEY (skill_id) REFERENCES skills(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS driftstones (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    skills_json TEXT
);

CREATE TABLE IF NOT EXISTS weapons (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    category TEXT NOT NULL,
    category_name TEXT NOT NULL,
    element TEXT NOT NULL,
    series TEXT NOT NULL,
    series_name TEXT,
    min_grade INTEGER NOT NULL,
    max_grade INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS weapon_levels (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    weapon_id TEXT NOT NULL,
    grade INTEGER NOT NULL,
    level INTEGER NOT NULL,
    name TEXT NOT NULL,
    attack INTEGER NOT NULL,
    element_attack INTEGER NOT NULL,
    affinity INTEGER NOT NULL,
    skills_json TEXT,
    UNIQUE (weapon_id, grade, level),
    FOREIGN KEY (weapon_id) REFERENCES weapons(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS weapon_skills (
    weapon_id TEXT NOT NULL,
    skill_id TEXT NOT NULL,
    unlock_grade INTEGER NOT NULL,
    skill_level INTEGER NOT NULL,
    PRIMARY KEY (weapon_id, skill_id, unlock_grade),
    FOREIGN KEY (weapon_id) REFERENCES weapons(id) ON DELETE CASCADE,
    FOREIGN KEY (skill_id) REFERENCES skills(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS armor_pieces (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    category TEXT NOT NULL,
    category_name TEXT NOT NULL,
    series TEXT NOT NULL,
    series_name TEXT,
    min_grade INTEGER NOT NULL,
    max_grade INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS armor_levels (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    armor_id TEXT NOT NULL,
    grade INTEGER NOT NULL,
    level INTEGER NOT NULL,
    name TEXT NOT NULL,
    defense INTEGER NOT NULL,
    driftsmelt_slots INTEGER NOT NULL,
    skills_json TEXT,
    UNIQUE (armor_id, grade, level),
    FOREIGN KEY (armor_id) REFERENCES armor_pieces(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS armor_skills (
    armor_id TEXT NOT NULL,
    skill_id TEXT NOT NULL,
    unlock_grade INTEGER NOT NULL,
    skill_level INTEGER NOT NULL,
    PRIMARY KEY (armor_id, skill_id, unlock_grade),
    FOREIGN KEY (armor_id) REFERENCES armor_pieces(id) ON DELETE CASCADE,
    FOREIGN KEY (skill_id) REFERENCES skills(id) ON DELETE CASCADE
);

-- Fast lookup indexes
CREATE INDEX IF NOT EXISTS idx_monsters_name ON monsters(name COLLATE NOCASE);
CREATE INDEX IF NOT EXISTS idx_monsters_slug ON monsters(slug);
CREATE INDEX IF NOT EXISTS idx_monster_materials_name ON monster_materials(name COLLATE NOCASE);
CREATE INDEX IF NOT EXISTS idx_skills_name ON skills(name COLLATE NOCASE);
CREATE INDEX IF NOT EXISTS idx_weapons_name ON weapons(name COLLATE NOCASE);
CREATE INDEX IF NOT EXISTS idx_weapons_category ON weapons(category);
CREATE INDEX IF NOT EXISTS idx_weapon_levels_name ON weapon_levels(name COLLATE NOCASE);
CREATE INDEX IF NOT EXISTS idx_armor_name ON armor_pieces(name COLLATE NOCASE);
CREATE INDEX IF NOT EXISTS idx_armor_levels_name ON armor_levels(name COLLATE NOCASE);
CREATE INDEX IF NOT EXISTS idx_armor_skills_skill ON armor_skills(skill_id);
CREATE INDEX IF NOT EXISTS idx_weapon_skills_skill ON weapon_skills(skill_id);
`

// OpenDB opens a SQLite database connection with optimal settings.
func OpenDB(dbPath string) (*sql.DB, error) {
	if err := os.MkdirAll(filepath.Dir(dbPath), 0755); err != nil {
		return nil, fmt.Errorf("create db dir: %w", err)
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("open sqlite db: %w", err)
	}

	if _, err := db.Exec(SchemaSQL); err != nil {
		db.Close()
		return nil, fmt.Errorf("init schema: %w", err)
	}

	return db, nil
}
