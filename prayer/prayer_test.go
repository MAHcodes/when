package prayer

import (
	"strings"
	"testing"

	"golang.org/x/net/html"
)

const sampleHTML = `<html><body><div class="mnr-salat-menu-wrapper">
<ul class="mnr-salat-times">
<li><span>الإمساك</span><span>03:46</span></li>
<li><span>صلاة الصبح</span><span>03:57</span></li>
<li><span>الشروق</span><span>05:28</span></li>
<li><span>الظهر</span><span>12:40</span></li>
<li><span>العصر</span><span>16:25</span></li>
<li><span>المغرب</span><span>20:13</span></li>
<li><span>العشاء</span><span>21:17</span></li>
<li><span>منتصف الليل</span><span>23:55</span></li>
</ul></div></body></html>`

func TestParsePrayerTimes(t *testing.T) {
	prayers, err := parsePrayerTimes(strings.NewReader(sampleHTML))
	if err != nil {
		t.Fatal(err)
	}

	if len(prayers) != 8 {
		t.Fatalf("expected 8 prayers, got %d", len(prayers))
	}

	expect := []struct {
		name string
		time string
	}{
		{"Imsak", "03:46"},
		{"Fajr", "03:57"},
		{"Shorouq", "05:28"},
		{"Dhuhr", "12:40"},
		{"Asr", "16:25"},
		{"Maghrib", "20:13"},
		{"Isha", "21:17"},
		{"Midnight", "23:55"},
	}

	for i, e := range expect {
		if prayers[i].Name != e.name {
			t.Errorf("prayer %d: expected name %q, got %q", i, e.name, prayers[i].Name)
		}
		if prayers[i].Time != e.time {
			t.Errorf("prayer %d: expected time %q, got %q", i, e.time, prayers[i].Time)
		}
	}
}

func TestParsePrayerTimesNoList(t *testing.T) {
	_, err := parsePrayerTimes(strings.NewReader("<html></html>"))
	if err == nil {
		t.Fatal("expected error for page without prayer list")
	}
}

func TestArabicToEnglishCoverage(t *testing.T) {
	for ar, en := range arabicToEnglish {
		if ar == "" {
			t.Error("empty Arabic key in arabicToEnglish")
		}
		if en == "" {
			t.Errorf("empty English value for Arabic key %q", ar)
		}
	}
}

func TestFind(t *testing.T) {
	prayers := []Prayer{
		{Name: "Fajr", Time: "03:57"},
		{Name: "Dhuhr", Time: "12:40"},
		{Name: "Maghrib", Time: "20:13"},
	}

	tests := []struct {
		query  string
		wantOk bool
		want   string
	}{
		{"Fajr", true, "03:57"},
		{"fajr", true, "03:57"},
		{"FAJR", true, "03:57"},
		{"Dhuhr", true, "12:40"},
		{"dhuhr", true, "12:40"},
		{"Maghrib", true, "20:13"},
		{"maghrib", true, "20:13"},
		{"  Fajr  ", true, "03:57"},
		{"unknown", false, ""},
		{"", false, ""},
	}

	for _, tt := range tests {
		p, ok := Find(prayers, tt.query)
		if ok != tt.wantOk {
			t.Errorf("Find(%q): got ok=%v, want %v", tt.query, ok, tt.wantOk)
			continue
		}
		if ok && p.Time != tt.want {
			t.Errorf("Find(%q): got time=%q, want %q", tt.query, p.Time, tt.want)
		}
	}
}

func TestNextEmpty(t *testing.T) {
	_, _, err := Next(nil)
	if err == nil {
		t.Fatal("expected error for empty prayers")
	}

	_, _, err = Next([]Prayer{})
	if err == nil {
		t.Fatal("expected error for empty prayers")
	}
}

func TestNextReturnsFromList(t *testing.T) {
	prayers := []Prayer{
		{Name: "Fajr", Time: "03:57"},
		{Name: "Dhuhr", Time: "12:40"},
		{Name: "Asr", Time: "16:25"},
		{Name: "Maghrib", Time: "20:13"},
	}

	p, dur, err := Next(prayers)
	if err != nil {
		t.Fatal(err)
	}

	if dur <= 0 {
		t.Errorf("expected positive duration, got %v", dur)
	}

	found := false
	for _, pp := range prayers {
		if pp.Name == p.Name && pp.Time == p.Time {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Next returned prayer %q %q not in input list", p.Name, p.Time)
	}
}

func TestNextWrapsToTomorrow(t *testing.T) {
	prayers := []Prayer{
		{Name: "Imsak", Time: "00:01"},
		{Name: "Fajr", Time: "00:02"},
	}

	p, dur, err := Next(prayers)
	if err != nil {
		t.Fatal(err)
	}

	if dur <= 0 {
		t.Errorf("expected positive duration, got %v", dur)
	}
	if p.Name != "Imsak" && p.Name != "Fajr" {
		t.Errorf("expected Imsak or Fajr, got %q", p.Name)
	}
}

func TestNextSkipsMalformedTimes(t *testing.T) {
	prayers := []Prayer{
		{Name: "Fajr", Time: "not-a-time"},
		{Name: "Dhuhr", Time: "12:40"},
	}

	p, dur, err := Next(prayers)
	if err != nil {
		t.Fatal(err)
	}

	if dur <= 0 {
		t.Errorf("expected positive duration, got %v", dur)
	}
	if p.Name != "Dhuhr" {
		t.Errorf("expected Dhuhr, got %q", p.Name)
	}
}

func TestNextAllMalformed(t *testing.T) {
	prayers := []Prayer{
		{Name: "Fajr", Time: "bad"},
	}

	_, _, err := Next(prayers)
	if err == nil {
		t.Fatal("expected error when all times are malformed")
	}
}

func TestTodayEntries(t *testing.T) {
	prayers := []Prayer{
		{Name: "Fajr", Time: "03:57"},
		{Name: "Dhuhr", Time: "12:40"},
		{Name: "Maghrib", Time: "20:13"},
	}

	entries, err := TodayEntries(prayers)
	if err != nil {
		t.Fatal(err)
	}

	if len(entries) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(entries))
	}

	for i, e := range entries {
		if e.Prayer.Name != prayers[i].Name {
			t.Errorf("entry %d: expected name %q, got %q", i, prayers[i].Name, e.Prayer.Name)
		}
	}
}

func TestTodayEntriesEmpty(t *testing.T) {
	_, err := TodayEntries(nil)
	if err == nil {
		t.Fatal("expected error for nil prayers")
	}

	_, err = TodayEntries([]Prayer{})
	if err == nil {
		t.Fatal("expected error for empty prayers")
	}
}

func TestTodayEntriesSkipsMalformed(t *testing.T) {
	prayers := []Prayer{
		{Name: "Fajr", Time: "not-a-time"},
		{Name: "Dhuhr", Time: "12:40"},
	}

	entries, err := TodayEntries(prayers)
	if err != nil {
		t.Fatal(err)
	}

	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].Prayer.Name != "Dhuhr" {
		t.Errorf("expected Dhuhr, got %q", entries[0].Prayer.Name)
	}
}

func TestTodayEntriesSorts(t *testing.T) {
	prayers := []Prayer{
		{Name: "Maghrib", Time: "20:13"},
		{Name: "Fajr", Time: "03:57"},
		{Name: "Dhuhr", Time: "12:40"},
	}

	entries, err := TodayEntries(prayers)
	if err != nil {
		t.Fatal(err)
	}

	if len(entries) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(entries))
	}

	expected := []string{"Fajr", "Dhuhr", "Maghrib"}
	for i, e := range entries {
		if e.Prayer.Name != expected[i] {
			t.Errorf("position %d: expected %q, got %q", i, expected[i], e.Prayer.Name)
		}
	}
}

func TestPrayersJSON(t *testing.T) {
	prayers := []Prayer{
		{Name: "Fajr", Time: "03:57"},
		{Name: "Dhuhr", Time: "12:40"},
	}

	got, err := PrayersJSON(prayers)
	if err != nil {
		t.Fatal(err)
	}

	expected := `[
  {
    "Name": "Fajr",
    "Time": "03:57"
  },
  {
    "Name": "Dhuhr",
    "Time": "12:40"
  }
]`
	if got != expected {
		t.Errorf("PrayersJSON mismatch\ngot:\n%s\n\nwant:\n%s", got, expected)
	}
}

func TestPrayersJSONEmpty(t *testing.T) {
	got, err := PrayersJSON([]Prayer{})
	if err != nil {
		t.Fatal(err)
	}
	if got != "[]" {
		t.Errorf("expected empty array, got %s", got)
	}
}

func TestPrayersTSV(t *testing.T) {
	prayers := []Prayer{
		{Name: "Fajr", Time: "03:57"},
		{Name: "Dhuhr", Time: "12:40"},
	}

	got := PrayersTSV(prayers)
	expected := "Fajr\t03:57\nDhuhr\t12:40\n"
	if got != expected {
		t.Errorf("PrayersTSV mismatch:\ngot:  %q\nwant: %q", got, expected)
	}
}

func TestSingleJSON(t *testing.T) {
	got := SingleJSON("Fajr", "03:57")
	expected := `{"name":"Fajr","time":"03:57"}`
	if got != expected {
		t.Errorf("SingleJSON mismatch:\ngot:  %s\nwant: %s", got, expected)
	}
}

func TestNextJSON(t *testing.T) {
	got := NextJSON("Maghrib", "20:13", "2h45m0s")
	expected := `{"name":"Maghrib","remaining":"2h45m0s","time":"20:13"}`
	if got != expected {
		t.Errorf("NextJSON mismatch:\ngot:  %s\nwant: %s", got, expected)
	}
}

func TestTextContent(t *testing.T) {
	r := strings.NewReader("<span>Hello <b>World</b></span>")
	doc, err := html.Parse(r)
	if err != nil {
		t.Fatal(err)
	}

	got := textContent(doc)
	if !strings.Contains(got, "Hello") || !strings.Contains(got, "World") {
		t.Errorf("textContent expected to contain 'Hello' and 'World', got %q", got)
	}
}
