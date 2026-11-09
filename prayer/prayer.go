package prayer

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"golang.org/x/net/html"
)

const baseURL = "https://almanar.com.lb/salat/"

var arabicToEnglish = map[string]string{
	"الإمساك":     "Imsak",
	"صلاة الصبح":  "Fajr",
	"الشروق":      "Shorouq",
	"الظهر":       "Dhuhr",
	"العصر":       "Asr",
	"المغرب":      "Maghrib",
	"العشاء":      "Isha",
	"منتصف الليل": "Midnight",
}

type Prayer struct {
	Name string
	Time string
}

// TimeEntry pairs a prayer with its time.Time for today.
type TimeEntry struct {
	Prayer Prayer
	Time   time.Time
}

// TodayEntries converts prayers to sorted time.Time entries for the current day.
func TodayEntries(prayers []Prayer) ([]TimeEntry, error) {
	now := time.Now()
	var entries []TimeEntry
	for _, p := range prayers {
		parts := strings.Split(p.Time, ":")
		if len(parts) != 2 {
			continue
		}
		h, m := 0, 0
		fmt.Sscanf(parts[0], "%d", &h)
		fmt.Sscanf(parts[1], "%d", &m)
		pt := time.Date(now.Year(), now.Month(), now.Day(), h, m, 0, 0, time.Local)
		entries = append(entries, TimeEntry{p, pt})
	}
	if len(entries) == 0 {
		return nil, fmt.Errorf("no prayer entries")
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Time.Before(entries[j].Time)
	})
	return entries, nil
}

type cacheData struct {
	Date   string   `json:"date"`
	Prayer []Prayer `json:"prayers"`
}

func cachePath() (string, error) {
	cacheDir, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(cacheDir, "when")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}
	return filepath.Join(dir, "prayer-times.json"), nil
}

func writeCache(prayers []Prayer) error {
	path, err := cachePath()
	if err != nil {
		return err
	}
	data := cacheData{
		Date:   time.Now().Format("2006-01-02"),
		Prayer: prayers,
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return json.NewEncoder(f).Encode(data)
}

func readCache() ([]Prayer, error) {
	path, err := cachePath()
	if err != nil {
		return nil, err
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var data cacheData
	if err := json.NewDecoder(f).Decode(&data); err != nil {
		return nil, err
	}

	today := time.Now().Format("2006-01-02")
	if data.Date != today {
		return nil, fmt.Errorf("cached data is from %s, not today", data.Date)
	}
	return data.Prayer, nil
}

func Fetch() ([]Prayer, error) {
	if cached, err := readCache(); err == nil {
		return cached, nil
	}
	return fetchRemote()
}

// Refresh ignores any cached data, refetches prayer times, and updates the cache.
func Refresh() ([]Prayer, error) {
	return fetchRemote()
}

func fetchRemote() ([]Prayer, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	client := &http.Client{Timeout: 15 * time.Second}

	var lastErr error
	for i := range 3 {
		if ctx.Err() != nil {
			return nil, fmt.Errorf("fetch timed out: %w", ctx.Err())
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL, nil)
		if err != nil {
			lastErr = fmt.Errorf("attempt %d: %w", i+1, err)
			continue
		}
		req.Header.Set("User-Agent", "when/1.0")

		resp, err := client.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("attempt %d: %w", i+1, err)
			time.Sleep(time.Duration(1<<i) * time.Second)
			continue
		}

		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			lastErr = fmt.Errorf("attempt %d: unexpected status %s", i+1, resp.Status)
			time.Sleep(time.Duration(1<<i) * time.Second)
			continue
		}

		prayers, err := parsePrayerTimes(resp.Body)
		resp.Body.Close()
		if err != nil {
			lastErr = fmt.Errorf("attempt %d: %w", i+1, err)
			time.Sleep(time.Duration(1<<i) * time.Second)
			continue
		}

		writeCache(prayers)
		return prayers, nil
	}

	return nil, fmt.Errorf("fetch failed after 3 retries: %w", lastErr)
}

func parsePrayerTimes(r io.Reader) ([]Prayer, error) {
	doc, err := html.Parse(r)
	if err != nil {
		return nil, fmt.Errorf("failed to parse HTML: %w", err)
	}

	var result []Prayer
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "ul" {
			for _, attr := range n.Attr {
				if attr.Key == "class" && strings.Contains(attr.Val, "mnr-salat-times") {
					result = extractTimes(n)
					return
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)

	if len(result) == 0 {
		return nil, fmt.Errorf("could not find prayer times on the page")
	}

	return result, nil
}

func extractTimes(ul *html.Node) []Prayer {
	var prayers []Prayer
	for li := ul.FirstChild; li != nil; li = li.NextSibling {
		if li.Type != html.ElementNode || li.Data != "li" {
			continue
		}

		arabicName := ""
		timeVal := ""
		idx := 0
		for span := li.FirstChild; span != nil; span = span.NextSibling {
			if span.Type == html.ElementNode && span.Data == "span" {
				text := strings.TrimSpace(textContent(span))
				if idx == 0 {
					arabicName = text
				} else if idx == 1 {
					timeVal = text
				}
				idx++
			}
		}

		if arabicName != "" && timeVal != "" {
			en := arabicToEnglish[arabicName]
			if en == "" {
				en = arabicName
			}
			prayers = append(prayers, Prayer{Name: en, Time: timeVal})
		}
	}
	return prayers
}

func textContent(n *html.Node) string {
	if n.Type == html.TextNode {
		return n.Data
	}
	var b strings.Builder
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		b.WriteString(textContent(c))
	}
	return b.String()
}

func Find(prayers []Prayer, query string) (*Prayer, bool) {
	q := strings.TrimSpace(query)
	for _, p := range prayers {
		if strings.EqualFold(p.Name, q) {
			return &Prayer{Name: p.Name, Time: p.Time}, true
		}
	}
	return nil, false
}

func Next(prayers []Prayer) (*Prayer, time.Duration, error) {
	if len(prayers) == 0 {
		return nil, 0, fmt.Errorf("no prayer times available")
	}

	entries, err := TodayEntries(prayers)
	if err != nil {
		return nil, 0, err
	}

	now := time.Now()
	for _, e := range entries {
		if e.Time.After(now) {
			return &Prayer{Name: e.Prayer.Name, Time: e.Prayer.Time}, e.Time.Sub(now), nil
		}
	}

	first := entries[0]
	tomorrow := time.Date(now.Year(), now.Month(), now.Day()+1, first.Time.Hour(), first.Time.Minute(), 0, 0, time.Local)
	return &Prayer{Name: first.Prayer.Name, Time: tomorrow.Format("15:04")}, tomorrow.Sub(now), nil
}

func PrayersJSON(prayers []Prayer) (string, error) {
	b, err := json.MarshalIndent(prayers, "", "  ")
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func PrayersTSV(prayers []Prayer) string {
	var b strings.Builder
	for _, p := range prayers {
		b.WriteString(p.Name)
		b.WriteByte('\t')
		b.WriteString(p.Time)
		b.WriteByte('\n')
	}
	return b.String()
}

func SingleJSON(name, time string) string {
	b, _ := json.Marshal(map[string]string{"name": name, "time": time})
	return string(b)
}

func NextJSON(name, time, remaining string) string {
	b, _ := json.Marshal(map[string]string{"name": name, "time": time, "remaining": remaining})
	return string(b)
}
