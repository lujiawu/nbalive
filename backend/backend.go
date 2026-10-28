package backend

// Game.Status values returned by 直播吧's list (state field).
const (
	StatusUpcoming = 1
	StatusLive     = 2
	StatusFinal    = 3
)

// Game is one game on the schedule.
type Game struct {
	GameId    string
	Date      string // "YYYY-MM-DD" (Beijing); the detail feed is keyed on it
	HomeTeam  string
	AwayTeam  string
	GameClock string // tip-off time, "第4节 10:31", or "完赛"
	HomeScore int
	AwayScore int
	TipOff    string   // "07:30", the scheduled start time (set in every state)
	Updated   string   // "11:02", when the score last changed (live/final only)
	AwayStars []string // away team's key players, "戴维斯 29分10板0助"
	HomeStars []string // home team's key players
	Status    int      // 1 = scheduled, 2 = live, 3 = final
}

// NotStarted reports whether the game is scheduled but hasn't tipped off yet
// (so there's no box score or play-by-play to fetch). GameClock holds the
// tip-off time in this state.
func (g Game) NotStarted() bool {
	return g.Status == StatusUpcoming
}

// IsLive reports whether the game is currently in progress, in which case its
// data is worth auto-refreshing.
func (g Game) IsLive() bool {
	return g.Status == StatusLive
}
