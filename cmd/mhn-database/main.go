package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/CFMVCarlos/mhn-database/pkg/crawler"
	"github.com/CFMVCarlos/mhn-database/pkg/database"
	"github.com/CFMVCarlos/mhn-database/pkg/mcp"
	"github.com/CFMVCarlos/mhn-database/pkg/query"
)

const (
	defaultRawDir = "data/raw"
	defaultDBPath = "data/mhn.db"
)

func printUsage() {
	fmt.Print(`mhn-database: Monster Hunter Now Knowledge Base & MCP Server

Usage:
  mhn-database <command> [arguments]

Commands:
  crawl       Crawl official site once, save raw JSON to data/raw, and populate SQLite DB
  build-db    Rebuild SQLite DB from cached raw JSON files (100% offline, no network)
  query       Look up information for human verification in terminal
  mcp-server  Run standard Model Context Protocol (stdio) server for AI assistants

Query Commands:
  mhn-database query monster <name|slug|id>
  mhn-database query weapon <name|id> [--grade N] [--level N]
  mhn-database query armor <name|id> [--grade N] [--level N]
  mhn-database query skill <name|id>
  mhn-database query material <name>

Flags for crawl:
  --force     Force re-crawl from network even if local cached data exists
  --raw-dir   Directory for raw JSON files (default: data/raw)
  --db        Path to SQLite database (default: data/mhn.db)

Flags for query:
  --json      Output raw JSON result
  --db        Path to SQLite database (default: data/mhn.db)
`)
}

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	command := os.Args[1]

	switch command {
	case "crawl":
		fs := flag.NewFlagSet("crawl", flag.ExitOnError)
		force := fs.Bool("force", false, "Force re-crawl even if cached raw files exist")
		rawDir := fs.String("raw-dir", defaultRawDir, "Directory to store raw JSON")
		dbPath := fs.String("db", defaultDBPath, "SQLite database destination path")
		fs.Parse(os.Args[2:])

		fmt.Println("=== Monster Hunter Now Data Crawler ===")
		if err := crawler.CrawlAll(*rawDir, *force); err != nil {
			fmt.Fprintf(os.Stderr, "Error crawling data: %v\n", err)
			os.Exit(1)
		}

		fmt.Println("=== Building SQLite Knowledge Base ===")
		if err := database.BuildDatabase(*rawDir, *dbPath); err != nil {
			fmt.Fprintf(os.Stderr, "Error building database: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("✓ Done! SQLite database is ready for queries and AI access.")

	case "build-db":
		fs := flag.NewFlagSet("build-db", flag.ExitOnError)
		rawDir := fs.String("raw-dir", defaultRawDir, "Directory containing raw JSON files")
		dbPath := fs.String("db", defaultDBPath, "SQLite database destination path")
		fs.Parse(os.Args[2:])

		fmt.Println("=== Building SQLite Knowledge Base (Offline) ===")
		if err := database.BuildDatabase(*rawDir, *dbPath); err != nil {
			fmt.Fprintf(os.Stderr, "Error building database: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("✓ Done! Database successfully rebuilt.")

	case "query":
		handleQuery(os.Args[2:])

	case "mcp-server":
		fs := flag.NewFlagSet("mcp-server", flag.ExitOnError)
		dbPath := fs.String("db", defaultDBPath, "SQLite database path")
		fs.Parse(os.Args[2:])

		db, err := database.OpenDB(*dbPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to open database %s: %v\n", *dbPath, err)
			os.Exit(1)
		}
		defer db.Close()

		if err := mcp.StartStdioServer(db); err != nil {
			fmt.Fprintf(os.Stderr, "MCP server terminated: %v\n", err)
			os.Exit(1)
		}

	case "help", "-h", "--help":
		printUsage()

	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n\n", command)
		printUsage()
		os.Exit(1)
	}
}

func handleQuery(args []string) {
	if len(args) < 1 {
		fmt.Println("Usage: mhn query <monster|weapon|armor|skill|material> <name> [--json] [--grade N] [--level N]")
		os.Exit(1)
	}

	entityType := args[0]
	fs := flag.NewFlagSet("query "+entityType, flag.ExitOnError)
	asJSON := fs.Bool("json", false, "Output results in JSON format")
	dbPath := fs.String("db", defaultDBPath, "SQLite database path")
	grade := fs.Int("grade", 0, "Specific grade (1 to 10)")
	level := fs.Int("level", 0, "Specific level (1 to 5)")
	fs.Parse(args[1:])

	restArgs := fs.Args()
	if len(restArgs) < 1 {
		fmt.Fprintf(os.Stderr, "Error: search name is required for 'mhn query %s'\n", entityType)
		os.Exit(1)
	}
	searchName := strings.Join(restArgs, " ")

	db, err := database.OpenDB(*dbPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to open database: %v\n", err)
		os.Exit(1)
	}
	defer db.Close()

	switch entityType {
	case "monster":
		res, err := query.LookupMonster(db, searchName)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Query failed: %v\n", err)
			os.Exit(1)
		}
		if *asJSON {
			fmt.Println(query.ToJSON(res))
			return
		}
		printMonster(res)

	case "weapon":
		res, err := query.LookupWeapon(db, searchName, *grade, *level)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Query failed: %v\n", err)
			os.Exit(1)
		}
		if *asJSON {
			fmt.Println(query.ToJSON(res))
			return
		}
		printWeapon(res)

	case "armor":
		res, err := query.LookupArmor(db, searchName, *grade, *level)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Query failed: %v\n", err)
			os.Exit(1)
		}
		if *asJSON {
			fmt.Println(query.ToJSON(res))
			return
		}
		printArmor(res)

	case "skill":
		res, err := query.LookupSkill(db, searchName)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Query failed: %v\n", err)
			os.Exit(1)
		}
		if *asJSON {
			fmt.Println(query.ToJSON(res))
			return
		}
		printSkill(res)

	case "material", "drops":
		res, err := query.LookupMaterial(db, searchName)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Query failed: %v\n", err)
			os.Exit(1)
		}
		if *asJSON {
			fmt.Println(query.ToJSON(res))
			return
		}
		printMaterial(res)

	default:
		fmt.Fprintf(os.Stderr, "Unknown query entity: %s\n", entityType)
		os.Exit(1)
	}
}

func printMonster(m *query.MonsterResult) {
	fmt.Printf("\n=== %s (ID: %s, Slug: %s) ===\n", m.Name, m.ID, m.Slug)
	if m.Species != "" {
		fmt.Printf("Species:     %s\n", m.Species)
	}
	if m.Habitats != "" {
		fmt.Printf("Habitats:    %s\n", m.Habitats)
	}
	if m.Elements != "" {
		fmt.Printf("Elements:    %s\n", m.Elements)
	}
	if len(m.Weaknesses) > 0 {
		fmt.Printf("Weaknesses:  %s\n", strings.Join(m.Weaknesses, ", "))
	}
	if m.Description != "" {
		fmt.Printf("\nDescription:\n  %s\n", m.Description)
	}
	if m.UsefulInfo != "" {
		fmt.Printf("\nUseful Info:\n  %s\n", m.UsefulInfo)
	}

	if len(m.BreakableParts) > 0 {
		fmt.Println("\nBreakable Parts:")
		for _, bp := range m.BreakableParts {
			fmt.Printf("  • %-20s (Type: %s)\n", bp.PartName, bp.BreakType)
		}
	}

	if len(m.Materials) > 0 {
		fmt.Println("\nObtainable Materials:")
		for _, mat := range m.Materials {
			fmt.Printf("  • Rarity %d (Min Grade %d★): %s\n", mat.Rarity, mat.MinGrade, mat.Name)
		}
	}
	fmt.Println()
}

func printWeapon(w *query.WeaponResult) {
	fmt.Printf("\n=== %s (Type: %s, Element: %s) ===\n", w.Name, w.CategoryName, w.Element)
	fmt.Printf("Series: %s | Grade Range: %d to %d\n", w.Series, w.MinGrade, w.MaxGrade)

	if len(w.Skills) > 0 {
		fmt.Println("\nUnlocked Skills:")
		for _, s := range w.Skills {
			fmt.Printf("  • [Grade %d] %s Lv %d\n", s.UnlockGrade, s.SkillName, s.SkillLevel)
		}
	}

	if len(w.Levels) > 0 {
		fmt.Printf("\nProgression Sample (%d levels recorded):\n", len(w.Levels))
		// Print first, middle, last
		samples := []query.WeaponLevelResult{w.Levels[0]}
		if len(w.Levels) > 2 {
			samples = append(samples, w.Levels[len(w.Levels)/2])
		}
		if len(w.Levels) > 1 {
			samples = append(samples, w.Levels[len(w.Levels)-1])
		}
		for _, l := range samples {
			fmt.Printf("  • Grade %d Lv %d: Attack %d | Element %d | Affinity %d%%\n",
				l.Grade, l.Level, l.Attack, l.ElementAttack, l.Affinity)
		}
	}
	fmt.Println()
}

func printArmor(a *query.ArmorResult) {
	fmt.Printf("\n=== %s (Slot: %s) ===\n", a.Name, a.CategoryName)
	fmt.Printf("Series: %s | Grade Range: %d to %d\n", a.Series, a.MinGrade, a.MaxGrade)

	if len(a.Skills) > 0 {
		fmt.Println("\nUnlocked Skills:")
		for _, s := range a.Skills {
			fmt.Printf("  • [Grade %d] %s Lv %d\n", s.UnlockGrade, s.SkillName, s.SkillLevel)
		}
	}

	if len(a.Levels) > 0 {
		fmt.Printf("\nDefense Progression Sample (%d levels recorded):\n", len(a.Levels))
		samples := []query.ArmorLevelResult{a.Levels[0]}
		if len(a.Levels) > 2 {
			samples = append(samples, a.Levels[len(a.Levels)/2])
		}
		if len(a.Levels) > 1 {
			samples = append(samples, a.Levels[len(a.Levels)-1])
		}
		for _, l := range samples {
			fmt.Printf("  • Grade %d Lv %d: Defense %d | Driftsmelt Slots: %d\n",
				l.Grade, l.Level, l.Defense, l.DriftsmeltSlots)
		}
	}
	fmt.Println()
}

func printSkill(s *query.SkillResult) {
	fmt.Printf("\n=== Skill: %s (Max Level: %d) ===\n", s.Name, s.MaxLevel)
	if len(s.Levels) > 0 {
		fmt.Println("\nLevel Effects:")
		for _, l := range s.Levels {
			fmt.Printf("  • Level %d: %s\n", l.Level, l.Description)
		}
	}

	if len(s.ArmorPieces) > 0 {
		fmt.Printf("\nArmor Pieces with this skill (%d pieces):\n", len(s.ArmorPieces))
		for _, g := range s.ArmorPieces {
			fmt.Printf("  • %-25s [%-5s] (Unlocked at Grade %d, grants Lv %d)\n",
				g.Name, g.Category, g.UnlockGrade, g.SkillLevel)
		}
	}

	if len(s.Weapons) > 0 {
		fmt.Printf("\nWeapons with this skill (%d weapons):\n", len(s.Weapons))
		for _, g := range s.Weapons {
			fmt.Printf("  • %-25s [%-15s] (Unlocked at Grade %d, grants Lv %d)\n",
				g.Name, g.Category, g.UnlockGrade, g.SkillLevel)
		}
	}
	fmt.Println()
}

func printMaterial(m *query.MaterialSourceResult) {
	fmt.Printf("\n=== Material: %s (Rarity %d★) ===\n", m.MaterialName, m.Rarity)
	fmt.Println("Dropped by:")
	for _, drop := range m.Monsters {
		fmt.Printf("  • %-20s (Min Monster Grade: %d★, Drop Rarity: %d)\n",
			drop.MonsterName, drop.MinGrade, drop.Rarity)
	}
	fmt.Println()
}
