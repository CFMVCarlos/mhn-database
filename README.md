# mhn-database

[![Go Report Card](https://goreportcard.com/badge/github.com/CFMVCarlos/mhn-database)](https://goreportcard.com/report/github.com/CFMVCarlos/mhn-database)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

High-performance Monster Hunter Now (MHN) SQLite database and Model Context Protocol (MCP) server for AI assistants and terminal queries.

Extracted from official Niantic/Capcom data: [monsterhunternow.com/en](https://monsterhunternow.com/en/).

---

## Key Features

- **⚡ Blazing Fast Go Implementation:** Concurrent HTTP/2 fetching, HTML-island JSON parsing, and embedded SQLite via `modernc.org/sqlite` (pure Go, zero CGO dependencies).
- **🔒 Strict "Crawl Once" Guarantee:** Never polls or runs background jobs. Fetches the official website once, caches raw snapshots in `data/raw/`, and strictly refuses to re-crawl from network unless `--force` is passed.
- **📦 100% Offline Database Rebuild:** Rebuild `data/mhn.db` offline from `data/raw/` at any time with `./bin/mhn-database build-db` without hitting the network.
- **🤖 Built-in MCP Server:** Standard Model Context Protocol (JSON-RPC stdio) server that lets LLMs / AI assistants directly invoke deterministic lookup tools (`get_monster`, `get_weapon`, `get_armor`, `get_skill`, `find_gear_by_skill`, `get_material_sources`, `run_sql`).
- **🔍 Terminal Query CLI:** Formatted CLI queries for quick terminal inspection with optional `--json` output.
- **🛡️ Rock-Solid Relational Schema:** Enforced foreign keys, indexed lookups, and dedicated join tables (`armor_skills`, `weapon_skills`, `monster_materials`, `monster_weaknesses`) preventing broken or hallucinated associations.

---

## Database Contents

- **Monsters (69):** Species, habitats, elemental affinities, weaknesses, breakable parts, tactics & lore.
- **Materials (451):** Monster drops with exact rarities (1★ - 6★) and minimum monster star unlock grades.
- **Weapons (721):** Weapons across 14 categories with all grades (1 to 10) and levels (1 to 5), attack, elemental stats, and affinity.
- **Armor Pieces (355):** Pieces across all slots (Head, Chest, Arms, Waist, Legs), series, defense progressions, and driftsmelt slots.
- **Skills (137):** Skill levels (1-5) with effect descriptions and equipment associations.
- **Driftstones (39):** Feystones and skill pools.

---

## Installation & Building

```bash
# Clone repository
git clone https://github.com/CFMVCarlos/mhn-database.git
cd mhn-database

# Build binary
go build -o bin/mhn-database ./cmd/mhn-database

# Build SQLite database (100% offline from cached data/raw/)
./bin/mhn-database build-db
```

---

## Usage

### 1. Offline Database Build (Recommended)
```bash
./bin/mhn-database build-db
```
Recreates tables and indexes strictly from `data/raw/` in ~1 second with zero network calls.

### 2. Crawl & Network Refresh
```bash
./bin/mhn-database crawl
```
- Fetches all official sections (`monsters`, `weapons`, `armor`, `skills`, plus monster lore and drops).
- Saves raw JSON snapshots to `data/raw/`.
- Builds and populates `data/mhn.db`.

If run again, it detects the existing cache in `data/raw/` and skips the network crawl:
```text
✓ Raw data already exists in data/raw. Skipping network crawl (strictly crawling once as configured; use --force to re-crawl).
```

To force a network refresh:
```bash
./bin/mhn-database crawl --force
```

---

### 3. Terminal Query CLI

#### Monster Lookup
```bash
./bin/mhn-database query monster Rathalos
./bin/mhn-database query monster "Zinogre" --json
```

#### Weapon Lookup
```bash
./bin/mhn-database query weapon "Wyvern Blade"
./bin/mhn-database query weapon "Buster Sword" --grade 8 --level 1
```

#### Armor Lookup
```bash
./bin/mhn-database query armor "Rathalos Mail"
./bin/mhn-database query armor "Kulu Vambraces" --grade 6
```

#### Skill Lookup (and gear that provides it)
```bash
./bin/mhn-database query skill "Critical Eye"
./bin/mhn-database query skill "Lock On"
```

#### Material Drop Sources
```bash
./bin/mhn-database query material "Rathalos Plate"
./bin/mhn-database query material "Wyvern Gem Shard"
```

---

### 4. AI Agent Integration via MCP (Model Context Protocol)

Start the stdio server:
```bash
./bin/mhn-database mcp-server
```

#### Adding to AI Agent Configs (Claude Desktop / Cursor / Antigravity):
```json
{
  "mcpServers": {
    "monster-hunter-now": {
      "command": "/absolute/path/to/mhn-database/bin/mhn-database",
      "args": ["mcp-server", "--db", "/absolute/path/to/mhn-database/data/mhn.db"]
    }
  }
}
```

#### Exposed MCP Tools:
| Tool | Description | Parameters |
|---|---|---|
| `get_monster` | Get monster weaknesses, breakable parts, habitats, and drops | `name_or_id` (string) |
| `get_weapon` | Get weapon attack, element, skills, and grade progression | `name_or_id` (string), `grade` (int?), `level` (int?) |
| `get_armor` | Get armor defense, slot, series, and unlocked skills | `name_or_id` (string), `grade` (int?), `level` (int?) |
| `get_skill` | Get skill level descriptions and all equipment bestowing it | `name_or_id` (string) |
| `find_gear_by_skill` | Search for all armor and weapons granting a skill | `skill_name` (string), `min_level` (int?) |
| `get_material_sources`| Find which monsters drop a material and at what rarity | `material_name` (string) |
| `run_sql` | Execute custom read-only SQL queries against the database | `query` (string) |

---

## Project Structure

```
├── cmd/
│   └── mhn-database/
│       └── main.go         # CLI entry point (crawl, build-db, query, mcp-server)
├── pkg/
│   ├── crawler/
│   │   ├── crawler.go      # Concurrent HTTP fetcher & HTML island extractor
│   │   └── crawler_test.go # Crawl integrity tests
│   ├── database/
│   │   ├── db.go           # SQLite connection & schema initialization
│   │   └── loader.go       # JSON transformation into normalized relational tables
│   ├── mcp/
│   │   ├── server.go       # JSON-RPC 2.0 stdio Model Context Protocol server
│   │   └── server_test.go  # MCP protocol unit tests
│   └── query/
│       ├── query.go        # Relational lookup engine & JSON serializer
│       └── query_test.go   # Entity lookup unit tests
├── data/
│   ├── raw/                # Cached JSON snapshots (monsters, armor, weapons, skills, details)
│   └── mhn.db              # Normalized SQLite database (generated, gitignored)
├── bin/
│   └── mhn-database        # Compiled Go binary
├── go.mod
└── go.sum
```

---

## License

MIT
