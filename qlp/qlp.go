package qlp

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"regexp"
)

// Match represents the information for a single match. It contains the total number of kills,
// the list of players and the number of kills for each player. Additionally, it contains the
// number of kills by means of death.
type Match struct {
	TotalKills   int            `json:"total_kills"`
	Players      []string       `json:"players"`
	Kills        map[string]int `json:"kills"`
	KillsByMeans map[string]int `json:"kills_by_means"`
}

// Matches implements a custom JSON marshaler interface in order to return the grouped
// information for each match according to the requirements. It is used instead of a regular
// map because marshaling a map does not guarantee the order of the elements.
type Matches []Match

func (m Matches) MarshalJSON() ([]byte, error) {
	buff := bytes.Buffer{}
	buff.WriteRune('{')

	for i, game := range m {
		buff.WriteString(fmt.Sprintf(`"game_%d":`, i+1)) // 1-indexed
		gameJSON, err := json.Marshal(game)
		if err != nil {
			return nil, err
		}
		buff.Write(gameJSON)

		hasNext := i < len(m)-1
		if hasNext {
			buff.WriteRune(',')
		}
	}

	buff.WriteRune('}')
	return buff.Bytes(), nil
}

// For more information about the patterns, I suggest trying them out at https://regex101.com
// The following patterns use non capturing groups in order to return only the relevant
// information.
var (
	// eventExpr matches the relevant events for the parser. These are InitGame and Kill.
	eventExpr = regexp.MustCompile(`^(?:\s+\d{1,2}:\d{2}\s)(InitGame|Kill)`)

	// killExpr matches the Kill events. Through the use of groups, they return the killer,
	// the victim and the means of death.
	killExpr = regexp.MustCompile(`^(?:\s+\d{1,2}:\d{2}\s)(?:Kill:\s[\d\s]+:\s)(.+)(?:\skilled\s)(.+)(?:\sby\s)(.+)`)
)

// ParseLog parses a Quake 3 log file and returns the information for each match.
func ParseLog(source io.Reader) (Matches, error) {
	scanner := bufio.NewScanner(source)
	var matches Matches

	if err := skipInitGameEvent(scanner); err != nil {
		return nil, fmt.Errorf("failed to read log file: %w", err)
	}

	currentMatch := newMatchInfo()
	for scanner.Scan() {
		line := scanner.Text()
		matched := eventExpr.FindStringSubmatch(line)
		if len(matched) == 0 {
			continue
		}

		event := matched[1]
		if event == "InitGame" {
			match := Match{
				TotalKills:   currentMatch.totalKills,
				Players:      currentMatch.getPlayerList(),
				Kills:        currentMatch.kills,
				KillsByMeans: currentMatch.killsByMeans,
			}
			matches = append(matches, match)
			currentMatch = newMatchInfo()
			continue
		}

		killEvent := killExpr.FindStringSubmatch(line)
		if len(killEvent) == 0 {
			return nil, fmt.Errorf("failed to find expected kill event at line %s", line)
		}

		killer, killed, killedBy := killEvent[1], killEvent[2], killEvent[3]
		currentMatch.registerKillEvent(killer, killed, killedBy)
	}

	// since we use InitGame as a separator, the last game is not included withing the loop
	match := Match{
		TotalKills:   currentMatch.totalKills,
		Players:      currentMatch.getPlayerList(),
		Kills:        currentMatch.kills,
		KillsByMeans: currentMatch.killsByMeans,
	}
	matches = append(matches, match)

	if scanner.Err() != nil {
		return nil, fmt.Errorf("failed to read log file: %w", scanner.Err())
	}

	return matches, nil
}

// matchInfo is an internal type used to store game data while parsing a log file.
type matchInfo struct {
	totalKills   int
	players      map[string]struct{}
	kills        map[string]int
	killsByMeans map[string]int
}

func newMatchInfo() *matchInfo {
	return &matchInfo{
		players:      make(map[string]struct{}),
		kills:        make(map[string]int),
		killsByMeans: make(map[string]int),
	}
}

// skipInitGameEvent skips an InitGame event. This is necessary because the log file contains
// a single "InitGame" event which is not preceded by a "ShutdownGame" event. Therefore we use
// InitGame, as opposed to ShutdownGame, to separate games. Due to this, the first game is not
// preceded by an InitGame event and must be skipped.
func skipInitGameEvent(scanner *bufio.Scanner) error {
	for scanner.Scan() {
		line := scanner.Text()
		if eventExpr.MatchString(line) {
			break
		}
	}

	return scanner.Err()
}

func (m *matchInfo) registerKillEvent(killer, killed, means string) {
	// TODO: verify whether players killing themselves should be counted as a kill

	for _, player := range [...]string{killer, killed} {
		if player == "<world>" {
			continue
		}

		m.players[player] = struct{}{}
		// the following is crucial to make sure even 0-kill players are included in the
		// kills map
		if _, ok := m.kills[player]; !ok {
			m.kills[player] = 0
		}
	}

	if killer == "<world>" {
		m.kills[killed]--
	} else {
		m.kills[killer]++
	}

	m.killsByMeans[means]++
}

func (m *matchInfo) getPlayerList() []string {
	players := make([]string, 0, len(m.players))
	for player := range m.players {
		players = append(players, player)
	}
	return players
}
