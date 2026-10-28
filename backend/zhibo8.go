package backend

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// 直播吧 (zhibo8) is the only data source. These are undocumented, plain-HTTP
// JSON endpoints on three different hosts; any of them can change without
// notice, so each one is referenced exactly once below.
const (
	listByDateURL = "https://bifen4m.qiumibao.com/json/%s/list.htm"
	boxURL        = "https://dc4pc.qiumibao.com/dc/matchs/data/%s/player_%s.htm"
	maxSidURL     = "https://dingshi4pc.qiumibao.com/livetext/data/cache/max_sid/%s/0.htm"
	playsURL      = "https://dingshi4pc.qiumibao.com/livetext/data/cache/livetext/%s/0/page_10/%d.htm"

	// nbaLeagueID is 直播吧's league id for the NBA.
	nbaLeagueID = "924"

	// playPages is how many 10-play pages the detail feed fetches (newest
	// first). A terminal screen holds about 27 lines, so 6 pages give roughly
	// two screens of scrollback; anything more would cost an extra request per
	// page on every refresh.
	// shortcut: only the newest 60 plays are shown; raise this when the
	// scrollback turns out to be too short in practice.
	playPages = 6

	// UserAgent is sent because the endpoints reject some client signatures.
	UserAgent = "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0 Safari/537.36"
)

var httpClient = &http.Client{Timeout: 10 * time.Second}

// Beijing returns the timezone the schedule is keyed on.
func Beijing() *time.Location {
	if loc, err := time.LoadLocation("Asia/Shanghai"); err == nil {
		return loc
	}
	return time.FixedZone("CST", 8*60*60)
}

// Today returns today's date ("2006-01-02") in Beijing time, which is how
// 直播吧 groups its daily schedule.
func Today() string {
	return time.Now().In(Beijing()).Format("2006-01-02")
}

func get(rawURL string) ([]byte, error) {
	req, err := http.NewRequest(http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", UserAgent)
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s: %s", rawURL, resp.Status)
	}
	return io.ReadAll(resp.Body)
}

func getJSON(rawURL string, v any) error {
	b, err := get(rawURL)
	if err != nil {
		return err
	}
	return json.Unmarshal(b, v)
}

// GetGamesForDate returns every NBA game played on the given date
// ("YYYY-MM-DD", Beijing time). The dated list is the full slate; the undated
// one (/json/list.htm) is only a rolling window that drops finished games, so
// it is not used.
func GetGamesForDate(date string) ([]Game, error) {
	b, err := get(fmt.Sprintf(listByDateURL, date))
	if err != nil {
		return nil, err
	}
	return parseGames(b)
}

// zAny decodes a JSON string or number into a string: 直播吧 mixes both for the
// same field (e.g. "state" is "3" for one game and 3 for another).
type zAny string

func (v *zAny) UnmarshalJSON(b []byte) error {
	s := strings.Trim(strings.TrimSpace(string(b)), `"`)
	if s == "null" {
		s = ""
	}
	*v = zAny(s)
	return nil
}

// Int reads the value as a whole number, or 0 if it isn't one ("", "-", "92.5").
func (v zAny) Int() int {
	n, _ := strconv.Atoi(strings.TrimSpace(string(v)))
	return n
}

// ma splits a "made-attempted" value such as "5-9".
func (v zAny) ma() (made, attempted int) {
	return splitMA(string(v))
}

// zGame is one entry of the daily list. The list covers every sport, so games
// are filtered down to basketball + the NBA.
type zGame struct {
	ID         zAny     `json:"id"`
	LeagueID   zAny     `json:"leagueid"`
	Type       string   `json:"type"`
	Date       string   `json:"sdate"`
	Time       string   `json:"time"`
	HomeTeam   string   `json:"home_team"`
	VisitTeam  string   `json:"visit_team"`
	HomeScore  zAny     `json:"home_score"`
	VisitScore zAny     `json:"visit_score"`
	PeriodCN   string   `json:"period_cn"`
	State      zAny     `json:"state"` // 1 = scheduled, 2 = live, 3 = final
	Update     string   `json:"update"`
	PlayerData zLineups `json:"player_data"`
}

// zLineups holds each side's top scorers as the list reports them; the list
// calls the away team "left" and the home team "right".
type zLineups struct {
	Left  []zStar `json:"left"`
	Right []zStar `json:"right"`
}

type zStar struct {
	Name  string `json:"player_name"`
	Value string `json:"value"` // "29分10板0助", sometimes prose like "膝盖受伤退场"
}

func parseGames(b []byte) ([]Game, error) {
	var resp struct {
		List []zGame `json:"list"`
	}
	if err := json.Unmarshal(b, &resp); err != nil {
		return nil, err
	}
	games := make([]Game, 0, len(resp.List))
	for _, g := range resp.List {
		if g.Type == "basketball" && string(g.LeagueID) == nbaLeagueID {
			games = append(games, g.toGame())
		}
	}
	return games, nil
}

func (g zGame) toGame() Game {
	return Game{
		GameId:    string(g.ID),
		Date:      g.Date,
		HomeTeam:  g.HomeTeam,
		AwayTeam:  g.VisitTeam,
		GameClock: g.clock(),
		HomeScore: g.HomeScore.Int(),
		AwayScore: g.VisitScore.Int(),
		TipOff:    g.Time,
		Updated:   hhmm(g.Update),
		AwayStars: topStars(g.PlayerData.Left),
		HomeStars: topStars(g.PlayerData.Right),
		Status:    g.State.Int(),
	}
}

// starLimit caps how many players the list row has room for.
const starLimit = 3

// topStars renders the feed's key players as "name stats", in the order 直播吧
// sends them. The list carries at most three per team; missing data yields nil.
func topStars(stars []zStar) []string {
	out := make([]string, 0, starLimit)
	for _, s := range stars {
		if s.Name == "" || len(out) == starLimit {
			break
		}
		if s.Value == "" {
			out = append(out, s.Name)
			continue
		}
		out = append(out, s.Name+" "+s.Value)
	}
	return out
}

// hhmm trims a wall-clock stamp to hours and minutes: "11:02:26" -> "11:02".
func hhmm(t string) string {
	if len(t) > 5 {
		return t[:5]
	}
	return t
}

// clock is the status column: tip-off time before the game, quarter and clock
// while live, "完赛" once it's over.
func (g zGame) clock() string {
	switch g.State {
	case "1":
		return g.Time
	case "3":
		return "完赛"
	default:
		return strings.Join(strings.Fields(g.PeriodCN), " ") // "第4节\n10:31"
	}
}

// splitMA splits a "made-attempted" field such as "5-9". A "-" or empty value
// yields zeros.
func splitMA(s string) (made, attempted int) {
	if i := strings.IndexByte(s, '-'); i >= 0 {
		made, attempted = atoi(s[:i]), atoi(s[i+1:])
	}
	return made, attempted
}

func atoi(s string) int {
	n, _ := strconv.Atoi(strings.TrimSpace(s))
	return n
}

// zPlayer is one player's line. 直播吧 reports shooting as "made-attempted".
type zPlayer struct {
	Name  string `json:"player_name_cn"`
	Min   string `json:"minutes"`
	Field zAny   `json:"field"`
	Three zAny   `json:"three"`
	Free  zAny   `json:"free"`
	Off   zAny   `json:"off"`
	Def   zAny   `json:"def"`
	Reb   zAny   `json:"off+def"`
	Ast   zAny   `json:"ass"`
	Stl   zAny   `json:"ste"`
	Blk   zAny   `json:"blo"`
	To    zAny   `json:"turn"`
	PF    zAny   `json:"fouls"`
	Pts   zAny   `json:"points"`
	PM    zAny   `json:"plusMinus"`
}

type zTeam struct {
	Name    string    `json:"team_name_cn"`
	Total   zPlayer   `json:"total"`
	Players []zPlayer `json:"on"`
}

// GetGameDetail fetches a game's box score and play-by-play. The play-by-play
// is best-effort: a missing feed must not hide the box score.
func GetGameDetail(id, date string) (GameDetail, error) {
	b, err := get(fmt.Sprintf(boxURL, date, id))
	if err != nil {
		return GameDetail{}, err
	}
	d, err := parseBox(b)
	if err != nil {
		return GameDetail{}, err
	}
	d.Plays = fetchPlays(id)
	return d, nil
}

func parseBox(b []byte) (GameDetail, error) {
	var resp struct {
		Data struct {
			// The feed calls the away team "host" and the home team "guest".
			Host  zTeam `json:"host"`
			Guest zTeam `json:"guest"`
		} `json:"data"`
	}
	if err := json.Unmarshal(b, &resp); err != nil {
		return GameDetail{}, err
	}
	return GameDetail{
		Away: resp.Data.Host.toTeamDetail(),
		Home: resp.Data.Guest.toTeamDetail(),
	}, nil
}

func (t zTeam) toTeamDetail() TeamDetail {
	tot := t.Total
	fgm, fga := tot.Field.ma()
	tpm, tpa := tot.Three.ma()
	ftm, fta := tot.Free.ma()
	td := TeamDetail{
		Name:   t.Name,
		Score:  tot.Pts.Int(),
		FGPct:  pct(fgm, fga),
		FG3Pct: pct(tpm, tpa),
		FTM:    ftm, FTA: fta,
		Reb: tot.Reb.Int(), Ast: tot.Ast.Int(), To: tot.To.Int(),
		FGM: fgm, FGA: fga, TPM: tpm, TPA: tpa,
		OReb: tot.Off.Int(), DReb: tot.Def.Int(),
		Stl: tot.Stl.Int(), Blk: tot.Blk.Int(), PF: tot.PF.Int(),
	}

	for _, p := range t.Players {
		pfm, pfa := p.Field.ma()
		ptm, pta := p.Three.ma()
		pftm, pfta := p.Free.ma()
		td.Players = append(td.Players, PlayerLine{
			Name: p.Name,
			Min:  p.Min,
			Pts:  p.Pts.Int(),
			Ast:  p.Ast.Int(),
			Reb:  p.Reb.Int(),
			Blk:  p.Blk.Int(),
			To:   p.To.Int(),
			// 直播吧 omits +/- for players who never left the bench.
			PlusMinus: p.PM.Int(),
			FGM:       pfm, FGA: pfa,
			TPM: ptm, TPA: pta,
			FTM: pftm, FTA: pfta,
			OReb: p.Off.Int(), DReb: p.Def.Int(),
			Stl: p.Stl.Int(), PF: p.PF.Int(),
		})
	}
	return td
}

// zPlay is one play-by-play record. There is no game clock and no team tag;
// live_time is the wall-clock time the event was posted.
type zPlay struct {
	Period string `json:"pid_text"`
	Time   string `json:"live_time"`
	Text   string `json:"live_text"`
}

// fetchPlays returns the newest-first play-by-play, walking back page by page.
// The page endpoint only accepts sids that are multiples of 10 (its page size),
// so the newest usable one is derived from max_sid.
func fetchPlays(id string) []PlayLine {
	b, err := get(fmt.Sprintf(maxSidURL, id))
	if err != nil {
		return nil
	}
	maxSid, err := strconv.Atoi(strings.TrimSpace(string(b)))
	if err != nil || maxSid < 10 {
		return nil
	}

	var plays []PlayLine
	for sid, page := maxSid/10*10, 0; sid > 0 && page < playPages; sid, page = sid-10, page+1 {
		pb, err := get(fmt.Sprintf(playsURL, id, sid))
		if err != nil {
			break
		}
		plays = append(plays, parsePlays(pb)...)
	}
	return plays
}

// parsePlays turns one page of records (oldest first) into newest-first lines.
func parsePlays(b []byte) []PlayLine {
	var recs []zPlay
	if err := json.Unmarshal(b, &recs); err != nil {
		return nil
	}
	plays := make([]PlayLine, 0, len(recs))
	for i := len(recs) - 1; i >= 0; i-- {
		plays = append(plays, PlayLine{
			When: strings.TrimSpace(recs[i].Period) + " " + clockOf(recs[i].Time),
			Desc: recs[i].Text,
		})
	}
	return plays
}

// clockOf takes the wall-clock time out of "2026-10-27 11:02:34".
func clockOf(t string) string {
	if i := strings.LastIndexByte(t, ' '); i >= 0 {
		return t[i+1:]
	}
	return t
}
