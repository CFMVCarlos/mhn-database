package crawler_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/CFMVCarlos/mhn-database/pkg/crawler"
)

func TestCrawlAll(t *testing.T) {
	tempDir := filepath.Join(os.TempDir(), "mhn_test_raw_full")
	defer os.RemoveAll(tempDir)

	err := crawler.CrawlAll(tempDir, false)
	if err != nil {
		t.Fatalf("CrawlAll failed: %v", err)
	}

	required := []string{
		"monsters.json",
		"skills.json",
		"armor.json",
		"weapons.json",
		"monster_details.json",
		"materials.json",
	}

	for _, f := range required {
		p := filepath.Join(tempDir, f)
		stat, err := os.Stat(p)
		if err != nil {
			t.Fatalf("expected file %s to exist: %v", f, err)
		}
		if stat.Size() == 0 {
			t.Fatalf("file %s is empty", f)
		}
		t.Logf("✓ %s (%d bytes)", f, stat.Size())
	}
}
