package ui

import (
	"testing"
	"time"

	"charm.land/lipgloss/v2"
	"github.com/lujiawu/nbalive/backend"
)

func TestPBPOnlyLayout(t *testing.T) {
	m := detail{width: 100, height: 30, pbpOnly: true}

	// 30 rows: 2 margins + 3 header + 1 blank + 1 hint + 1 spacer leaves 22;
	// the box spends 4 on border/title/divider.
	if got := m.pbpVisible(); got != 18 {
		t.Errorf("pbpVisible = %d, want 18", got)
	}
	// The feed-only box spans the row: 100 columns including its frame.
	if got := lipgloss.Width(m.renderInfoColumn(100, m.pbpBoxHeight(), true)); got != 100 {
		t.Errorf("feed box width = %d, want 100", got)
	}
}

func TestPBPFillsTheBodyHeight(t *testing.T) {
	// With few players the tables are shorter than the screen; the feed still
	// fills the body instead of shrinking to the left column.
	m := detail{width: 100, height: 30}
	if got := m.pbpBoxHeight(); got != m.playerColumnBudget() {
		t.Errorf("pbpBoxHeight = %d, want the body budget %d", got, m.playerColumnBudget())
	}
}

func TestPBPOnlyClampsScroll(t *testing.T) {
	m := detail{width: 100, height: 30}
	m.game.plays = make([]playEvent, 60)
	m.pbpScroll = m.maxScroll() // scrolled to the end in the two-column layout
	m.pbpOnly = true
	m.clampScrolls()
	if m.pbpScroll != m.maxScroll() || m.pbpScroll > 60-m.pbpVisible() {
		t.Errorf("pbpScroll = %d, want the feed-only limit %d", m.pbpScroll, m.maxScroll())
	}
}

func TestRefreshLabel(t *testing.T) {
	tests := []struct {
		name string
		m    detail
		want string
	}{
		{"finished game", detail{}, ""},
		{"refreshing", detail{live: true, refreshing: true}, "刷新中…"},
		{"manual refresh of a finished game", detail{refreshing: true}, "刷新中…"},
		{"counting down", detail{live: true, countdown: 12 * time.Second}, "刷新 12s"},
		{"partial second rounds up", detail{live: true, countdown: 300 * time.Millisecond}, "刷新 1s"},
		{"clamped at zero", detail{live: true, countdown: -5 * time.Second}, "刷新 0s"},
	}
	for _, tt := range tests {
		if got := tt.m.refreshLabel(); got != tt.want {
			t.Errorf("%s: refreshLabel() = %q, want %q", tt.name, got, tt.want)
		}
	}
}

func TestAnchorPlay(t *testing.T) {
	plays := []playEvent{
		{when: "第4节 11:00:03", desc: "new"},
		{when: "第4节 11:00:02", desc: "b"},
		{when: "第4节 11:00:01", desc: "a"},
	}
	if got := anchorPlay(plays, playKey(plays[2])); got != 2 {
		t.Errorf("anchorPlay = %d, want 2", got)
	}
	// A play that scrolled out of the feed falls back to the newest line.
	if got := anchorPlay(plays, "第4节 10:00:00\x00gone"); got != 0 {
		t.Errorf("anchorPlay = %d, want 0", got)
	}
}

func TestReplaceGameVanishedAnchorFallsBackToTop(t *testing.T) {
	m := detail{width: 140, height: 60, live: true}
	m.game.plays = []playEvent{{when: "old", desc: "gone"}}
	m.pbpScroll = 0

	m = m.replaceGame(backend.GameDetail{Plays: []backend.PlayLine{{When: "new", Desc: "play"}}})

	if m.pbpScroll != 0 {
		t.Errorf("pbpScroll = %d, want 0", m.pbpScroll)
	}
}
