package crawler

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
)

// TargetPage represents an official website section with its root island component.
type TargetPage struct {
	Name      string
	URL       string
	Component string
}

var TargetPages = []TargetPage{
	{
		Name:      "monsters",
		URL:       "https://monsterhunternow.com/en/monsters",
		Component: "SortableMonsterList",
	},
	{
		Name:      "skills",
		URL:       "https://monsterhunternow.com/en/skills",
		Component: "SortableSkillList",
	},
	{
		Name:      "armor",
		URL:       "https://monsterhunternow.com/en/armor",
		Component: "SortableArmorList",
	},
	{
		Name:      "weapons",
		URL:       "https://monsterhunternow.com/en/weapons",
		Component: "SortableWeaponList",
	},
}

// MonsterDetail holds lore and material translations extracted from individual monster pages.
type MonsterDetail struct {
	Slug         string            `json:"slug"`
	Description  string            `json:"description"`
	UsefulInfo   string            `json:"useful_info"`
	Translations map[string]string `json:"translations"`
}

// FetchAndExtractIsland retrieves the page HTML and extracts the JSON props of the given component.
func FetchAndExtractIsland(client *http.Client, url, component string) ([]byte, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("create request failed: %w", err)
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http get failed for %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status %d from %s", resp.StatusCode, url)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading body failed: %w", err)
	}

	compTag := []byte(fmt.Sprintf(`component="%s"`, component))
	idx := bytes.Index(body, compTag)
	if idx == -1 {
		return nil, fmt.Errorf("component %q not found in HTML from %s", component, url)
	}

	propsMarker := []byte(`props="`)
	propsStartIdx := bytes.Index(body[idx:], propsMarker)
	if propsStartIdx == -1 {
		return nil, fmt.Errorf("props attribute not found for component %q", component)
	}
	start := idx + propsStartIdx + len(propsMarker)

	quoteIdx := bytes.IndexByte(body[start:], '"')
	if quoteIdx == -1 {
		return nil, fmt.Errorf("closing quote for props attribute not found")
	}
	end := start + quoteIdx

	rawEscaped := string(body[start:end])
	unescaped := html.UnescapeString(rawEscaped)

	return []byte(unescaped), nil
}

// FetchMonsterDetails fetches lore and materials for all monster slugs concurrently.
func FetchMonsterDetails(client *http.Client, slugs []string) (map[string]MonsterDetail, map[string]string, error) {
	details := make(map[string]MonsterDetail)
	masterTranslations := make(map[string]string)
	var mu sync.Mutex

	sem := make(chan struct{}, 10)
	var wg sync.WaitGroup
	var fetchErr error

	for _, s := range slugs {
		wg.Add(1)
		go func(slug string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			url := fmt.Sprintf("https://monsterhunternow.com/en/monsters/%s", slug)
			req, err := http.NewRequest("GET", url, nil)
			if err != nil {
				return
			}
			req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")

			resp, err := client.Do(req)
			if err != nil {
				return
			}
			defer resp.Body.Close()

			body, err := io.ReadAll(resp.Body)
			if err != nil {
				return
			}

			// Extract ObtainableMaterials props for material translations
			matProps, err := extractIslandFromBody(body, "ObtainableMaterials")
			var transMap map[string]string
			if err == nil {
				var parsed struct {
					GuideTranslations map[string]string `json:"guideTranslations"`
				}
				if json.Unmarshal(matProps, &parsed) == nil {
					transMap = parsed.GuideTranslations
				}
			}

			// Extract lore/description from static HTML
			desc, usefulInfo := extractLoreFromHTML(string(body))

			mu.Lock()
			details[slug] = MonsterDetail{
				Slug:         slug,
				Description:  desc,
				UsefulInfo:   usefulInfo,
				Translations: transMap,
			}
			for k, v := range transMap {
				masterTranslations[k] = v
			}
			mu.Unlock()
		}(s)
	}

	wg.Wait()
	if fetchErr != nil {
		return nil, nil, fetchErr
	}

	return details, masterTranslations, nil
}

func extractIslandFromBody(body []byte, component string) ([]byte, error) {
	compTag := []byte(fmt.Sprintf(`component="%s"`, component))
	idx := bytes.Index(body, compTag)
	if idx == -1 {
		return nil, fmt.Errorf("component %q not found", component)
	}
	propsMarker := []byte(`props="`)
	propsStartIdx := bytes.Index(body[idx:], propsMarker)
	if propsStartIdx == -1 {
		return nil, fmt.Errorf("props attribute not found")
	}
	start := idx + propsStartIdx + len(propsMarker)
	quoteIdx := bytes.IndexByte(body[start:], '"')
	if quoteIdx == -1 {
		return nil, fmt.Errorf("closing quote not found")
	}
	end := start + quoteIdx
	unescaped := html.UnescapeString(string(body[start:end]))
	return []byte(unescaped), nil
}

// extractLoreFromHTML parses the text sections for Useful Information and description
func extractLoreFromHTML(htmlContent string) (string, string) {
	// Strip script and style tags
	reScript := regexp.MustCompile(`(?s)<script.*?</script>|<style.*?</style>`)
	cleaned := reScript.ReplaceAllString(htmlContent, " ")

	// Replace HTML tags with newlines
	reTag := regexp.MustCompile(`<[^>]+>`)
	text := reTag.ReplaceAllString(cleaned, "\n")

	lines := strings.Split(text, "\n")
	var trimmed []string
	for _, l := range lines {
		t := strings.TrimSpace(l)
		if t != "" {
			trimmed = append(trimmed, t)
		}
	}

	var desc, usefulInfo string
	for i, l := range trimmed {
		if strings.EqualFold(l, "Back to Monsters") && i+2 < len(trimmed) {
			// Monster name is usually i+1, description is i+2
			if trimmed[i+2] != "Monster Size" && !strings.HasPrefix(trimmed[i+2], "http") {
				desc = trimmed[i+2]
			}
		}
		if strings.EqualFold(l, "Useful Information") && i+1 < len(trimmed) {
			var uLines []string
			for j := i + 1; j < len(trimmed); j++ {
				if strings.EqualFold(trimmed[j], "Breakable Parts") || strings.EqualFold(trimmed[j], "Obtainable Materials") {
					break
				}
				uLines = append(uLines, trimmed[j])
			}
			usefulInfo = strings.Join(uLines, " ")
		}
	}

	return desc, usefulInfo
}

// CrawlAll fetches all sections and stores them in rawDir.
// If force is false and all files exist, it will NOT touch the network.
func CrawlAll(rawDir string, force bool) error {
	if err := os.MkdirAll(rawDir, 0755); err != nil {
		return fmt.Errorf("create raw dir: %w", err)
	}

	requiredFiles := []string{
		"monsters.json",
		"skills.json",
		"armor.json",
		"weapons.json",
		"monster_details.json",
		"materials.json",
	}

	allExist := true
	for _, f := range requiredFiles {
		filePath := filepath.Join(rawDir, f)
		if _, err := os.Stat(filePath); os.IsNotExist(err) {
			allExist = false
			break
		}
	}

	if allExist && !force {
		fmt.Printf("✓ Raw data already exists in %s. Skipping network crawl (strictly crawling once as configured; use --force to re-crawl).\n", rawDir)
		return nil
	}

	client := &http.Client{
		Timeout: 45 * time.Second,
	}

	// 1. Fetch main 4 pages
	var monstersData []byte
	for _, p := range TargetPages {
		filePath := filepath.Join(rawDir, p.Name+".json")
		if !force {
			if data, err := os.ReadFile(filePath); err == nil && len(data) > 0 {
				fmt.Printf("• %s.json already exists, skipping.\n", p.Name)
				if p.Name == "monsters" {
					monstersData = data
				}
				continue
			}
		}

		fmt.Printf("→ Fetching %s from %s...\n", p.Name, p.URL)
		data, err := FetchAndExtractIsland(client, p.URL, p.Component)
		if err != nil {
			return fmt.Errorf("failed fetching %s: %w", p.Name, err)
		}

		if err := os.WriteFile(filePath, data, 0644); err != nil {
			return fmt.Errorf("failed saving %s: %w", filePath, err)
		}
		fmt.Printf("✓ Saved %s (%d bytes)\n", filePath, len(data))

		if p.Name == "monsters" {
			monstersData = data
		}
	}

	// 2. Fetch individual monster detail pages for full lore & material translations
	detFile := filepath.Join(rawDir, "monster_details.json")
	matFile := filepath.Join(rawDir, "materials.json")

	if force || !fileExists(detFile) || !fileExists(matFile) {
		if len(monstersData) == 0 {
			var err error
			monstersData, err = os.ReadFile(filepath.Join(rawDir, "monsters.json"))
			if err != nil {
				return fmt.Errorf("failed to read monsters.json: %w", err)
			}
		}

		var mList struct {
			Monsters []struct {
				Slug string `json:"slug"`
			} `json:"monsters"`
		}
		if err := json.Unmarshal(monstersData, &mList); err != nil {
			return fmt.Errorf("failed to parse monsters json: %w", err)
		}

		var slugs []string
		for _, m := range mList.Monsters {
			if m.Slug != "" {
				slugs = append(slugs, m.Slug)
			}
		}

		fmt.Printf("→ Fetching details & material translations for %d monsters...\n", len(slugs))
		details, masterTranslations, err := FetchMonsterDetails(client, slugs)
		if err != nil {
			return fmt.Errorf("failed to fetch monster details: %w", err)
		}

		detJSON, _ := json.MarshalIndent(details, "", "  ")
		if err := os.WriteFile(detFile, detJSON, 0644); err != nil {
			return fmt.Errorf("failed to write %s: %w", detFile, err)
		}

		matJSON, _ := json.MarshalIndent(masterTranslations, "", "  ")
		if err := os.WriteFile(matFile, matJSON, 0644); err != nil {
			return fmt.Errorf("failed to write %s: %w", matFile, err)
		}

		fmt.Printf("✓ Saved monster details (%d records) and materials (%d translations)\n", len(details), len(masterTranslations))
	}

	return nil
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		return false
	}
	return !info.IsDir() && info.Size() > 0
}
