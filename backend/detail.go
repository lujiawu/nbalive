package backend

// PlayerLine is one player's box-score line.
type PlayerLine struct {
	Name string
	Min  string // minutes played ("24:45"; "00:00" for DNPs)

	Pts, Ast, Reb, Blk, To int
	PlusMinus              int

	// expanded detail
	FGM, FGA, TPM, TPA, FTM, FTA int
	OReb, DReb, Stl, PF          int
}

// TeamDetail is one team's full box score plus the aggregate stats shown in the
// comparison bar.
type TeamDetail struct {
	Name    string // Chinese team name ("灰熊")
	Score   int
	Players []PlayerLine

	FGPct, FG3Pct float64 // shooting percentages, 0-100
	FTM, FTA      int     // free throws made / attempted
	Reb, Ast, To  int

	// expanded detail
	FGM, FGA, TPM, TPA       int
	OReb, DReb, Stl, Blk, PF int
}

// PlayLine is a single play-by-play event. 直播吧 has no game clock and no team
// tag, so When carries the period and wall-clock time ("第4节 11:02:34").
type PlayLine struct {
	When string
	Desc string
}

// GameDetail is everything the detail screen needs for one game.
type GameDetail struct {
	Away, Home TeamDetail
	Plays      []PlayLine // newest first
}

// pct returns made/attempted as a 0-100 percentage, or 0 when none attempted.
func pct(made, attempted int) float64 {
	if attempted == 0 {
		return 0
	}
	return float64(made) / float64(attempted) * 100
}
