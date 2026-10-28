# NBALive

A terminal UI for following NBA games, box scores, and Chinese play-by-play.

Start on today's games, open any game for its full box score and 中文 play-by-play,
or jump to another date to look at past results. Live games refresh themselves
while you watch.

## Data

All data comes from [直播吧](https://www.zhibo8.cc/)'s public JSON endpoints
(`bifen4m` / `dc4pc` / `dingshi4pc` on `qiumibao.com`): the daily schedule,
per-game box scores, and the Chinese play-by-play feed.

These are unofficial, undocumented endpoints, so there is **no API key or
account required**, but they can change or rate-limit without notice. Live games
auto-refresh every 30 seconds.

Team names, player names and play-by-play are Chinese; the UI chrome is English.

## Install

From source:

```bash
git clone https://github.com/lujiawu/nbalive.git
cd nbalive
go build -o nbalive
sudo mv nbalive /usr/local/bin/
```

Or:

```bash
go install github.com/lujiawu/nbalive@latest
```

## Usage

```bash
nbalive
```

The app opens on today's games (Beijing time). Everything is keyboard-driven.
Each row shows the score and status, followed by the players 直播吧 lists for
each team (up to three, away side on the left):

```
奇才      103 - 110      黄蜂   完赛
戴维斯 29分10板0助         米勒 39分6板1助
杨     22分6板10助         里德 18分12板1助
```

### Game list

| Key | Action |
| --- | --- |
| `↑`/`k`, `↓`/`j` | Move between games |
| `enter` | Open the selected game |
| `←`/`h`, `→`/`l` | Previous / next day |
| `d` | Jump to a specific date |
| `/` | Filter games (team or player name) |
| `r` | Refresh immediately |
| `q` | Quit |

### Game details

| Key | Action |
| --- | --- |
| `tab`/`←`/`→` | Switch between the away box, home box, and play-by-play |
| `↑`/`k`, `↓`/`j` | Scroll the focused panel |
| `o` | Toggle expanded stats |
| `f` | Toggle the full-width play-by-play view |
| `r` | Refresh immediately |
| `q`/`esc` | Back to the game list |

Live games refresh on their own every 30 seconds; the play-by-play header counts
down to the next update.

## Provenance and license

This project is a derivative of [courtside](https://github.com/NolanFogarty/courtside)
by Nolan Fogarty. The original code carries no license grant, so it remains
**© Nolan Fogarty, all rights reserved**; the full upstream history is kept in
this repository (`git remote add upstream https://github.com/NolanFogarty/courtside.git`
and diff against it to see what changed).

Changes made in this repository — the 直播吧 backend, the Chinese data model, the
rewritten list and detail views, and their tests — are released under the MIT
License. See [LICENSE](LICENSE) for the exact scope.

Not affiliated with or endorsed by Nolan Fogarty or the original courtside project.
