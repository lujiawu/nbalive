package ui

import (
	"time"

	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"
	"github.com/lujiawu/nbalive/backend"
)

const (
	listview int = iota
	detailview
)

// heartbeat drives the countdown; the re-fetch itself fires once
// refreshInterval has elapsed since the last response landed.
const heartbeat = time.Second

// refreshInterval is how often live data (the list while viewing today, and an
// in-progress game's detail) is re-fetched. 直播吧 posts play-by-play roughly
// every few seconds, so 30s keeps the feed current without polling hard.
const refreshInterval = 30 * time.Second

// refreshTickMsg is the periodic heartbeat that drives the countdown and
// auto-refresh.
type refreshTickMsg struct{}

func refreshTick() tea.Cmd {
	return tea.Tick(heartbeat, func(time.Time) tea.Msg { return refreshTickMsg{} })
}

type root struct {
	current       int
	list          gamelist
	detail        detail
	width, height int
	lastRefresh   time.Time // when the last response arrived
	refreshing    bool      // a fetch is in flight; don't stack another
}

func newRoot(games []backend.Game) root {
	return root{
		current:     listview,
		list:        newGamesList(games),
		lastRefresh: time.Now(), // main.go already loaded today's games
	}
}

// nextRefreshIn is how long until the active view should be re-fetched.
func (r root) nextRefreshIn() time.Duration {
	return refreshInterval - time.Since(r.lastRefresh)
}

// fetch loads the active view's data. Unless forced, each view decides for
// itself whether auto-refresh applies (the list only refreshes today, the
// detail only a live game).
func (r root) fetch(force bool) tea.Cmd {
	if r.current == detailview {
		return r.detail.refreshCmd(force)
	}
	return r.list.refreshCmd(force)
}

// reload handles the manual refresh key: it bypasses the auto-refresh gates and
// marks a fetch as in flight. Repeated presses are ignored until it lands.
func (r root) reload() (root, tea.Cmd) {
	if r.refreshing {
		return r, nil
	}
	cmd := r.fetch(true)
	if cmd == nil {
		return r, nil
	}
	r.refreshing = true
	// Show the in-flight marker right away; the heartbeat clears it on the next
	// tick, so it stays visible for a moment even when the fetch is instant.
	r.detail.refreshing = true
	if r.current == listview {
		r.list = r.list.startLoad()
	}
	return r, cmd
}

func (r root) Init() tea.Cmd {
	return tea.Batch(r.list.Init(), refreshTick())
}

func (r root) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// Quit from anywhere
	if key, ok := msg.(tea.KeyPressMsg); ok && key.String() == "ctrl+c" {
		return r, tea.Quit
	}

	// Keep all sub-views sized regardless of which is active.
	if ws, ok := msg.(tea.WindowSizeMsg); ok {
		r.width, r.height = ws.Width, ws.Height
		updated, cmd := r.list.Update(ws)
		r.list = updated.(gamelist)
		r.detail, _ = r.detail.Update(ws)
		return r, cmd
	}

	// Heartbeat: refresh the countdown the detail view displays, and re-fetch the
	// active view once the interval has passed. A single 1s lineage keeps the
	// countdown smooth without ever stacking two polls.
	if _, ok := msg.(refreshTickMsg); ok {
		left := r.nextRefreshIn()
		r.detail.countdown = left
		r.detail.refreshing = r.refreshing

		if !r.refreshing && left <= 0 {
			if cmd := r.fetch(false); cmd != nil {
				r.refreshing = true
				return r, tea.Batch(cmd, refreshTick())
			}
			// Nothing to fetch (a past date, or a finished game): restart the
			// clock so the countdown doesn't sit at zero.
			r.lastRefresh = time.Now()
		}
		return r, refreshTick()
	}

	// Any response restarts the countdown, success or failure, so a flaky
	// endpoint doesn't wedge the timer.
	switch msg.(type) {
	case gamesLoadedMsg, gameDetailMsg:
		r.lastRefresh = time.Now()
		r.refreshing = false
	}

	// Day navigation messages always belong to the list, even if the user has
	// since opened the detail view.
	switch msg.(type) {
	case gamesLoadedMsg, daySettledMsg:
		updated, cmd := r.list.Update(msg)
		r.list = updated.(gamelist)
		return r, cmd
	}

	switch r.current {
	case listview:
		// Enter opens the selected game; "r" re-fetches the viewed date. Both are
		// ignored while the list is filtering or the user is typing a date.
		if key, ok := msg.(tea.KeyPressMsg); ok &&
			r.list.list.FilterState() != list.Filtering && !r.list.entering {
			switch key.String() {
			case "enter":
				if sel, ok := r.list.list.SelectedItem().(item); ok {
					prev := r.detail
					r.detail = newDetail(sel.game, r.width, r.height)
					r.detail.pbpOnly = prev.pbpOnly // keep the layout the user picked
					r.detail.countdown = r.nextRefreshIn()
					r.detail.refreshing = true // Init() is about to fetch
					r.current = detailview
					r.refreshing = true
					return r, r.detail.Init() // kick off the async fetch
				}
			case "r":
				return r.reload()
			}
		}

		updated, cmd := r.list.Update(msg)
		r.list = updated.(gamelist)
		return r, cmd

	case detailview:
		if key, ok := msg.(tea.KeyPressMsg); ok {
			switch key.String() {
			case "esc", "q":
				r.current = listview
				return r, nil
			case "r":
				return r.reload()
			}
		}

		var cmd tea.Cmd
		r.detail, cmd = r.detail.Update(msg)
		return r, cmd
	}

	return r, nil
}

func (r root) View() tea.View {
	switch r.current {
	case detailview:
		return r.detail.View()
	default:
		return r.list.View()
	}
}
