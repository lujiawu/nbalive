package backend

import (
	"os"
	"testing"
)

// fixture reads a recorded response from testdata.
func fixture(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatalf("read fixture %s: %v", name, err)
	}
	return b
}

func TestParseGames(t *testing.T) {
	games, err := parseGames(fixture(t, "list.json"))
	if err != nil {
		t.Fatal(err)
	}
	if len(games) == 0 {
		t.Fatal("no NBA games parsed")
	}
	for _, g := range games {
		if g.GameId == "" || g.HomeTeam == "" || g.AwayTeam == "" || g.GameClock == "" {
			t.Errorf("incomplete game: %+v", g)
		}
		if g.Date == "" {
			t.Errorf("game %s has no date", g.GameId)
		}
	}

	// The list also carries other leagues and sports; none may leak through.
	for _, g := range games {
		if g.GameId == "" {
			t.Error("parsed a non-NBA entry")
		}
	}

	var live, final, upcoming int
	for _, g := range games {
		switch {
		case g.IsLive():
			live++
		case g.NotStarted():
			upcoming++
		default:
			final++
		}
	}
	if live+final+upcoming != len(games) {
		t.Errorf("games with unknown status: %d", len(games)-(live+final+upcoming))
	}
	if final == 0 {
		t.Error("expected at least one final game in the fixture")
	}
}

// The list row shows more than the score: tip-off, last update, and each
// side's top scorer all come from the same response.
func TestParseGamesExtras(t *testing.T) {
	games, err := parseGames(fixture(t, "list.json"))
	if err != nil {
		t.Fatal(err)
	}
	byID := make(map[string]Game, len(games))
	for _, g := range games {
		byID[g.GameId] = g
	}

	g, ok := byID["2139835"]
	if !ok {
		t.Fatal("game 2139835 missing from fixture")
	}
	if g.TipOff != "07:30" {
		t.Errorf("TipOff = %q, want 07:30", g.TipOff)
	}
	if g.Updated != "10:24" {
		t.Errorf("Updated = %q, want 10:24 (seconds trimmed)", g.Updated)
	}
	if got := g.AwayStars; len(got) != 3 || got[0] != "欧文 18分6板6助" {
		t.Errorf("AwayStars = %v, want the away side's three players", got)
	}
	if got := g.HomeStars; len(got) != 3 || got[0] != "阿德托昆博 29分14板11助" {
		t.Errorf("HomeStars = %v, want the home side's three players", got)
	}
}

func TestParseGamesWithoutExtras(t *testing.T) {
	// Upcoming games have no scorers and no update time yet.
	data := []byte(`{"list":[
		{"id":7,"leagueid":924,"type":"basketball","sdate":"2026-11-01","time":"09:00",
		 "home_team":"火箭","visit_team":"老鹰","home_score":"","visit_score":"",
		 "period_cn":"","state":"1"}]}`)
	games, err := parseGames(data)
	if err != nil {
		t.Fatal(err)
	}
	if len(games) != 1 {
		t.Fatalf("got %d games, want 1", len(games))
	}
	g := games[0]
	if !g.NotStarted() {
		t.Errorf("Status = %d, want upcoming", g.Status)
	}
	if g.TipOff != "09:00" || g.Updated != "" || len(g.AwayStars) != 0 || len(g.HomeStars) != 0 {
		t.Errorf("unexpected extras: %+v", g)
	}
}

func TestParseGamesNumericFields(t *testing.T) { // 直播吧 sends the same field as a string for one game and a number for
	// another; both must parse.
	data := []byte(`{"list":[
		{"id":1,"leagueid":924,"type":"basketball","sdate":"2026-10-27","time":"09:00",
		 "home_team":"火箭","visit_team":"老鹰","home_score":120,"visit_score":"110",
		 "period_cn":"完赛","state":3}]}`)
	games, err := parseGames(data)
	if err != nil {
		t.Fatal(err)
	}
	if len(games) != 1 {
		t.Fatalf("got %d games, want 1", len(games))
	}
	g := games[0]
	if g.GameId != "1" || g.HomeScore != 120 || g.AwayScore != 110 {
		t.Errorf("unexpected game: %+v", g)
	}
	if g.Status != StatusFinal {
		t.Errorf("status = %d, want %d (final)", g.Status, StatusFinal)
	}
}

func TestGameClock(t *testing.T) {
	tests := []struct {
		g    zGame
		want string
	}{
		{zGame{State: "1", Time: "09:00"}, "09:00"},
		{zGame{State: "2", PeriodCN: "第4节\n10:31"}, "第4节 10:31"},
		{zGame{State: "3", PeriodCN: "完赛"}, "完赛"},
	}
	for _, tt := range tests {
		if got := tt.g.clock(); got != tt.want {
			t.Errorf("clock() = %q, want %q", got, tt.want)
		}
	}
}

func TestParseBox(t *testing.T) {
	d, err := parseBox(fixture(t, "player.json"))
	if err != nil {
		t.Fatal(err)
	}
	if d.Away.Name == "" || d.Home.Name == "" {
		t.Fatalf("missing team names: %q vs %q", d.Away.Name, d.Home.Name)
	}
	for _, team := range []TeamDetail{d.Away, d.Home} {
		if len(team.Players) == 0 {
			t.Fatalf("%s: no players", team.Name)
		}
		p := team.Players[0]
		if p.Name == "" || p.Min == "" {
			t.Errorf("%s: incomplete player line %+v", team.Name, p)
		}
		// Shooting comes as "made-attempted"; both halves must survive parsing.
		if p.FGA == 0 && p.FGM == 0 && team.FGA == 0 {
			t.Errorf("%s: shooting not parsed", team.Name)
		}
	}
	if d.Away.Score == 0 || d.Home.Score == 0 {
		t.Errorf("missing scores: %d-%d", d.Away.Score, d.Home.Score)
	}
	if d.Away.Reb != d.Away.OReb+d.Away.DReb {
		t.Errorf("%s: rebounds %d != %d+%d", d.Away.Name, d.Away.Reb, d.Away.OReb, d.Away.DReb)
	}
}

func TestSplitMA(t *testing.T) {
	tests := []struct {
		in        string
		made, att int
	}{
		{"5-9", 5, 9},
		{"0-0", 0, 0},
		{"-", 0, 0},
		{"", 0, 0},
	}
	for _, tt := range tests {
		made, att := splitMA(tt.in)
		if made != tt.made || att != tt.att {
			t.Errorf("splitMA(%q) = %d,%d, want %d,%d", tt.in, made, att, tt.made, tt.att)
		}
	}
}

func TestParsePlays(t *testing.T) {
	plays := parsePlays(fixture(t, "plays.json"))
	if len(plays) < 2 {
		t.Fatalf("got %d plays", len(plays))
	}
	if plays[0].When == "" || plays[0].Desc == "" {
		t.Errorf("incomplete play: %+v", plays[0])
	}
	// Newest first: the feed's last record must come out on top.
	if plays[0].When != "第4节 11:02:05" {
		t.Errorf("newest play = %q, want 第4节 11:02:05", plays[0].When)
	}
}
