package ui

import (
	"fmt"
	"io"
	"strings"
	"time"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/list"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/lujiawu/nbalive/backend"
)

var docStyle = lipgloss.NewStyle().Margin(1, 2)

type item struct {
	game backend.Game
}

func (i item) FilterValue() string {
	g := i.game
	return strings.Join(append(append([]string{g.AwayTeam, g.HomeTeam}, g.AwayStars...), g.HomeStars...), " ")
}

type gamelist struct {
	list     list.Model
	day      time.Time // the date currently being viewed
	seq      int       // bumped on each day change; debounces rapid navigation
	loading  bool
	err      error
	entering bool // "go to date" input mode
	input    textinput.Model
	height   int // terminal height, for bottom-anchoring the empty state
}

// daySettledMsg fires after a short pause following a day change; the fetch only
// runs if seq still matches (i.e. no newer keypress superseded it).
type daySettledMsg struct{ seq int }

// gamesLoadedMsg carries the result of an async day fetch.
type gamesLoadedMsg struct {
	day   time.Time
	games []backend.Game
	err   error
}

// fetchDebounce is how long to wait after the last day change before fetching.
const fetchDebounce = 250 * time.Millisecond

func (m gamelist) Init() tea.Cmd {
	return nil
}

func (m gamelist) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// Keep the list sized in every mode.
	if ws, ok := msg.(tea.WindowSizeMsg); ok {
		m.height = ws.Height
		h, v := docStyle.GetFrameSize()
		m.list.SetSize(ws.Width-h, ws.Height-v)
	}

	// While typing a date, the input owns every message (keys to edit, others
	// to drive the cursor blink).
	if m.entering {
		if key, ok := msg.(tea.KeyPressMsg); ok {
			if key.String() == "ctrl+c" {
				return m, tea.Quit
			}
			return m.updateDateInput(key)
		}
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(msg)
		return m, cmd
	}

	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		if msg.String() == "ctrl+c" {
			return m, tea.Quit
		}
		// Day navigation (unless typing a filter).
		if m.list.FilterState() != list.Filtering {
			switch msg.String() {
			case "d":
				return m.startDateInput()
			case "left", "h":
				return m.gotoDay(-1)
			case "right", "l":
				return m.gotoDay(1)
			}
		}
	case daySettledMsg:
		if msg.seq != m.seq {
			return m, nil // superseded by a newer day change; don't fetch
		}
		return m, fetchGames(m.day)
	case gamesLoadedMsg:
		if !sameDay(msg.day, m.day) {
			return m, nil // a newer day was requested; ignore stale result
		}
		wasLoading := m.loading
		m.loading = false
		var cmd tea.Cmd
		if msg.err != nil {
			// Only surface an error on a user-initiated load; ignore transient
			// auto-refresh failures and keep showing the current games.
			if wasLoading {
				m.err = msg.err
			}
		} else {
			m.err = nil
			cmd = m.applyGames(msg.games)
		}
		m.list.Title = m.title()
		return m, cmd
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

// gotoDay moves the viewed date by delta days immediately, but defers the fetch
// behind a debounce so spamming the key doesn't hammer the API.
func (m gamelist) gotoDay(delta int) (gamelist, tea.Cmd) {
	next := m.day.AddDate(0, 0, delta)
	// Don't navigate past today (ET) — there are no future results to show.
	if delta > 0 && afterToday(next) {
		return m, nil
	}
	m.day = next
	m.loading = true
	m.err = nil
	m.seq++
	m.list.Title = m.title()
	seq := m.seq
	return m, tea.Tick(fetchDebounce, func(time.Time) tea.Msg {
		return daySettledMsg{seq: seq}
	})
}

// startDateInput enters "go to date" mode, focusing a fresh text input.
func (m gamelist) startDateInput() (gamelist, tea.Cmd) {
	m.entering = true
	m.input.Reset()
	cmd := m.input.Focus()
	return m, cmd
}

// updateDateInput handles keys while typing a date: Enter jumps, Esc cancels,
// everything else edits the field.
func (m gamelist) updateDateInput(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.entering = false
		m.input.Blur()
		return m, nil
	case "enter":
		day, ok := parseDate(m.input.Value())
		m.entering = false
		m.input.Blur()
		if !ok {
			return m, nil // invalid input; stay on the current day
		}
		return m.jumpToDay(day)
	}

	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

// jumpToDay switches directly to an absolute date and fetches it immediately
// (no debounce — the user explicitly asked for this date).
func (m gamelist) jumpToDay(day time.Time) (gamelist, tea.Cmd) {
	m.day = day
	m.loading = true
	m.err = nil
	m.seq++ // invalidate any pending day-navigation debounce ticks
	m.list.Title = m.title()
	return m, fetchGames(m.day)
}

// parseDate accepts a few common date formats; year-less forms assume the
// current (ET) year.
func parseDate(s string) (time.Time, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}, false
	}
	for _, layout := range []string{"2006-01-02", "1/2/2006", "01/02/2006", "Jan 2 2006", "Jan 2, 2006"} {
		if t, err := time.Parse(layout, s); err == nil {
			return t, true
		}
	}
	for _, layout := range []string{"1/2", "01/02", "Jan 2"} {
		if t, err := time.Parse(layout, s); err == nil {
			return time.Date(bjToday().Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC), true
		}
	}
	return time.Time{}, false
}

func fetchGames(day time.Time) tea.Cmd {
	return func() tea.Msg {
		games, err := backend.GetGamesForDate(day.Format("2006-01-02"))
		return gamesLoadedMsg{day: day, games: games, err: err}
	}
}

// refreshCmd re-fetches the viewed date. Auto-refresh only applies to today
// (past days are static); a manual refresh forces it for any date.
func (m gamelist) refreshCmd(force bool) tea.Cmd {
	if m.entering || (!force && !sameDay(m.day, bjToday())) {
		return nil
	}
	return fetchGames(m.day)
}

// startLoad marks a user-initiated load so the title shows "(loading…)" and any
// failure is surfaced, the same way a date change does.
func (m gamelist) startLoad() gamelist {
	m.loading = true
	m.err = nil
	m.list.Title = m.title()
	return m
}

// applyGames replaces the list's items with the games for the current day.
func (m *gamelist) applyGames(games []backend.Game) tea.Cmd {
	items := make([]list.Item, len(games))
	for i, g := range games {
		items[i] = item{game: g}
	}
	return m.list.SetItems(items)
}

func (m gamelist) title() string {
	label := m.day.Format("Mon Jan 2")
	if m.day.Year() != bjToday().Year() {
		label = m.day.Format("Mon Jan 2, 2006") // past seasons are reachable
	}
	if sameDay(m.day, bjToday()) {
		label = "Today · " + label
	}
	t := "NBA Scores — " + label
	switch {
	case m.loading:
		t += "  (loading…)"
	case m.err != nil:
		t += "  (failed to load)"
	}
	return t
}

func sameDay(a, b time.Time) bool {
	return a.Year() == b.Year() && a.YearDay() == b.YearDay()
}

// afterToday reports whether t's calendar date is after today (ET). Both sides
// are normalized to their calendar date so the comparison ignores time of day
// and location differences.
func afterToday(t time.Time) bool {
	today := bjToday()
	ty, tm, td := t.Date()
	ny, nm, nd := today.Date()
	return time.Date(ty, tm, td, 0, 0, 0, 0, time.UTC).
		After(time.Date(ny, nm, nd, 0, 0, 0, 0, time.UTC))
}

// bjToday returns the current date in Beijing time, which is how 直播吧 keys
// its daily schedule.
func bjToday() time.Time {
	return time.Now().In(backend.Beijing())
}

func (m gamelist) View() tea.View {
	// While entering a date, show the input prompt in the title area. Drop the
	// Title chip's background so the prompt and typed text render with one
	// consistent style (this mirrors how the list draws its own filter input).
	if m.entering {
		m.list.Styles.Title = lipgloss.NewStyle()
		m.list.Title = m.input.View()
	}

	var content string
	if len(m.list.Items()) == 0 {
		// Hide the list entirely when empty; show just the title and a single
		// message (the bubbles list would otherwise render "No items." twice).
		note := "No games on this date."
		if m.loading {
			note = "Loading…"
		}
		top := lipgloss.JoinVertical(
			lipgloss.Left,
			m.list.Styles.Title.Render(m.list.Title),
			"",
			lipgloss.NewStyle().Faint(true).Render(note),
		)
		hint := renderHints(
			[2]string{"←/h", "prev day"},
			[2]string{"→/l", "next day"},
			[2]string{"d", "date"},
			[2]string{"r", "refresh"},
			[2]string{"q", "quit"},
		)

		// Push the hint to the bottom of the screen.
		_, vFrame := docStyle.GetFrameSize()
		spacerH := m.height - vFrame - lipgloss.Height(top) - lipgloss.Height(hint)
		if spacerH < 1 {
			spacerH = 1
		}
		spacer := lipgloss.NewStyle().Height(spacerH).Render("")
		content = lipgloss.JoinVertical(lipgloss.Left, top, spacer, hint)
	} else {
		content = m.list.View()
	}

	v := tea.NewView(docStyle.Render(content))
	v.AltScreen = true
	return v
}

// wTeam is the display width shared by the list rows and the detail scoreboard
// header. Chinese team names are at most four characters (8 columns).
const wTeam = 8

// scoreLine renders "AWAY 103 - 110 HOME" with fixed display widths, so the two
// views line up with each other.
func scoreLine(away string, awayScore int, home string, homeScore int) string {
	return pad(truncate(away, wTeam), wTeam, false) +
		fmt.Sprintf(" %3d - %-3d ", awayScore, homeScore) +
		pad(truncate(home, wTeam), wTeam, true)
}

// vsLine is the pre-game scoreboard: "AWAY  vs  HOME" in the same column
// widths as scoreLine, so rows line up before and after tip-off.
func vsLine(away, home string) string {
	return pad(truncate(away, wTeam), wTeam, false) + "    vs     " +
		pad(truncate(home, wTeam), wTeam, true)
}

// statusSuffix annotates the scoreboard. The second line already carries
// per-game content, so status and freshness live on the first line.
func statusSuffix(g backend.Game) string {
	if g.NotStarted() {
		return ""
	}
	if g.IsLive() {
		s := "   ● " + g.GameClock
		if g.Updated != "" {
			s += " · 更新 " + g.Updated
		}
		return s
	}
	return "   完赛"
}

// gameRowHeight is the lines per game: the scoreboard plus three player rows.
const gameRowHeight = 4

// wStarCol is the display width of each team's player column.
const wStarCol = 28

// gameLines is one list row: the scoreboard with status, then the players the
// list reports for each team, away side on the left and home on the right.
func gameLines(g backend.Game) [gameRowHeight]string {
	var lines [gameRowHeight]string
	if g.NotStarted() {
		lines[0] = vsLine(g.AwayTeam, g.HomeTeam)
		lines[1] = g.TipOff + " 开赛"
		return lines
	}

	lines[0] = scoreLine(g.AwayTeam, g.AwayScore, g.HomeTeam, g.HomeScore) + statusSuffix(g)
	for i := 0; i < gameRowHeight-1; i++ {
		var away, home string
		if i < len(g.AwayStars) {
			away = g.AwayStars[i]
		}
		if i < len(g.HomeStars) {
			home = g.HomeStars[i]
		}
		if away == "" && home == "" {
			continue // a team with fewer players just leaves the row blank
		}
		lines[i+1] = pad(truncate(away, wStarCol-2), wStarCol, false) + truncate(home, wStarCol)
	}
	return lines
}

// gameDelegate draws the multi-line row. The stock bubbles delegate only has
// room for a title and a description, so this repeats its selection, dimming,
// and truncation behaviour with more lines.
type gameDelegate struct {
	styles list.DefaultItemStyles
}

func newGameDelegate() gameDelegate {
	return gameDelegate{styles: list.NewDefaultItemStyles(true)}
}

func (d gameDelegate) Height() int                       { return gameRowHeight }
func (d gameDelegate) Spacing() int                      { return 0 }
func (gameDelegate) Update(tea.Msg, *list.Model) tea.Cmd { return nil }

func (d gameDelegate) Render(w io.Writer, m list.Model, index int, listItem list.Item) {
	it, ok := listItem.(item)
	if !ok || m.Width() <= 0 {
		return
	}

	titleStyle, descStyle := d.styles.NormalTitle, d.styles.NormalDesc
	switch {
	case m.FilterState() == list.Filtering && m.FilterValue() == "":
		titleStyle, descStyle = d.styles.DimmedTitle, d.styles.DimmedDesc
	case index == m.Index() && m.FilterState() != list.Filtering:
		titleStyle, descStyle = d.styles.SelectedTitle, d.styles.SelectedDesc
	}

	// The selected style draws a left border, so leave a column spare.
	width := m.Width() - d.styles.NormalTitle.GetPaddingLeft() -
		d.styles.NormalTitle.GetPaddingRight() - 1
	if width < 4 {
		width = 4
	}

	lines := gameLines(it.game)
	fmt.Fprint(w, titleStyle.Render(truncate(lines[0], width)))
	for _, line := range lines[1:] {
		fmt.Fprint(w, "\n", descStyle.Render(truncate(line, width)))
	}
}

func newGamesList(games []backend.Game) gamelist {
	items := make([]list.Item, len(games))
	for i, g := range games {
		items[i] = item{game: g}
	}

	ti := textinput.New()
	ti.Prompt = "Go to date (YYYY-MM-DD): "
	ti.CharLimit = 16

	m := gamelist{
		list:  list.New(items, newGameDelegate(), 0, 0),
		day:   bjToday(),
		input: ti,
	}

	// Repurpose ←/→ and h/l for day navigation; keep paging on pgup/pgdown and
	// free up "d" (a default next-page key) for the date jump.
	m.list.KeyMap.PrevPage.SetKeys("pgup", "b")
	m.list.KeyMap.NextPage.SetKeys("pgdown", "f")
	dayKeys := []key.Binding{
		key.NewBinding(key.WithKeys("left", "h"), key.WithHelp("←/h", "prev day")),
		key.NewBinding(key.WithKeys("right", "l"), key.WithHelp("→/l", "next day")),
		key.NewBinding(key.WithKeys("d"), key.WithHelp("d", "date")),
		key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "refresh")),
	}
	m.list.AdditionalShortHelpKeys = func() []key.Binding { return dayKeys }
	m.list.AdditionalFullHelpKeys = func() []key.Binding { return dayKeys }

	m.list.Title = m.title()
	return m
}

func Run(games []backend.Game) error {
	m := newRoot(games)
	p := tea.NewProgram(m)
	_, err := p.Run()
	return err
}
