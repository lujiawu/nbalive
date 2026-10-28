package ui

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/lujiawu/nbalive/backend"
)

func TestGameLines(t *testing.T) {
	final := backend.Game{
		AwayTeam: "黄蜂", HomeTeam: "奇才", AwayScore: 110, HomeScore: 103,
		GameClock: "完赛", TipOff: "07:00", Status: backend.StatusFinal,
		AwayStars: []string{"欧文 18分6板6助", "弗拉格 29分6板11助", "华盛顿 20分9板2助"},
		HomeStars: []string{"阿德托昆博 29分14板11助", "波蒂斯 10分12板2助", "米切尔 21分0板2助"},
	}

	tests := []struct {
		name  string
		g     backend.Game
		wants []string // substrings expected on the scoreboard line
	}{
		{"final", final, []string{"110 - 103", "完赛"}},
		{"live", backend.Game{
			AwayTeam: "掘金", HomeTeam: "勇士", AwayScore: 52, HomeScore: 55,
			GameClock: "第2节 02:25", Updated: "11:02", Status: backend.StatusLive,
		}, []string{"52 - 55", "● 第2节 02:25", "更新 11:02"}},
		{"upcoming", backend.Game{
			AwayTeam: "湖人", HomeTeam: "凯尔特人", GameClock: "09:00", TipOff: "09:00",
			Status: backend.StatusUpcoming,
		}, []string{"vs", "湖人", "凯尔特人"}},
	}
	for _, tt := range tests {
		lines := gameLines(tt.g)
		for _, want := range tt.wants {
			if !strings.Contains(lines[0], want) {
				t.Errorf("%s: line 0 %q missing %q", tt.name, lines[0], want)
			}
		}
	}

	// Every player line has the away entry first and the home entry at the
	// fixed column, so both sides line up down the list.
	lines := gameLines(final)
	for i := 0; i < 3; i++ {
		prefix, _, ok := strings.Cut(lines[i+1], final.HomeStars[i])
		if !ok {
			t.Fatalf("line %d %q missing home player %q", i+1, lines[i+1], final.HomeStars[i])
		}
		if got := lipgloss.Width(prefix); got != wStarCol {
			t.Errorf("line %d: home column starts at %d, want %d", i+1, got, wStarCol)
		}
	}

	// A team with fewer players leaves the remaining rows blank instead of
	// shifting the other side.
	short := backend.Game{
		Status:    backend.StatusFinal,
		AwayStars: []string{"甲 10分", "乙 8分"},
		HomeStars: []string{"丙 9分"},
	}
	lines = gameLines(short)
	if !strings.Contains(lines[1], "甲 10分") || !strings.Contains(lines[1], "丙 9分") {
		t.Errorf("line 1 = %q, want both sides", lines[1])
	}
	if !strings.Contains(lines[2], "乙 8分") {
		t.Errorf("line 2 = %q, want the away side's second player", lines[2])
	}
	if lines[3] != "" {
		t.Errorf("line 3 = %q, want blank", lines[3])
	}
}

func TestGameLinesUpcoming(t *testing.T) {
	g := backend.Game{
		AwayTeam: "湖人", HomeTeam: "凯尔特人", GameClock: "09:00", TipOff: "09:00",
		Status: backend.StatusUpcoming,
	}
	lines := gameLines(g)
	if lines[1] != "09:00 开赛" {
		t.Errorf("line 1 = %q, want the tip-off", lines[1])
	}
	if lines[2] != "" || lines[3] != "" {
		t.Errorf("expected blank player rows, got %q / %q", lines[2], lines[3])
	}
}

func TestFilterValueMatchesPlayerNames(t *testing.T) {
	it := item{game: backend.Game{
		AwayTeam: "黄蜂", HomeTeam: "奇才",
		AwayStars: []string{"欧文 18分"},
		HomeStars: []string{"戴维斯 29分"},
	}}
	for _, want := range []string{"黄蜂", "奇才", "欧文", "戴维斯"} {
		if !strings.Contains(it.FilterValue(), want) {
			t.Errorf("FilterValue %q missing %q", it.FilterValue(), want)
		}
	}
}
