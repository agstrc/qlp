// Package qlp implements a Quake log parser for the Quake III Arena game. The parser reads
// a log file and returns the information for each match.
package qlp

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"regexp"
	"slices"
	"strings"
)

// Match represents the information for a single match.
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

// MarshalJSON customizes the JSON representation of Matches. It returns a JSON object
// with the keys "game_1", "game_2", etc. for each match.
func (matches Matches) MarshalJSON() ([]byte, error) {
	buff := bytes.Buffer{}
	buff.WriteRune('{')

	for i, game := range matches {
		buff.WriteString(fmt.Sprintf(`"game_%d":`, i+1)) // 1-indexed
		gameJSON, err := json.Marshal(game)
		if err != nil {
			return nil, err
		}
		buff.Write(gameJSON)

		hasNext := i < len(matches)-1
		if hasNext {
			buff.WriteRune(',')
		}
	}

	buff.WriteRune('}')
	return buff.Bytes(), nil
}

// lineHeaderExpr is a regular expression to match the header of each line in the log file.
// It handles a special case of the log format found at the example, which is
//
//	26  0:00 ------------------------------------------------------------
//
// e.g. " 0:00 InitGame: \n", it matches " 0:00 ".
var lineHeaderExpr = regexp.MustCompile(`^\s*\d+:\d+\s|^\s*[\d\s:]+`)

// ParseLog reads and parses the log from an io.Reader, returning a slice of Matches or an error.
func ParseLog(log io.Reader) (Matches, error) {
	scanner := bufio.NewScanner(log)
	parser := newLogParser()

	currentLine := 0
	for scanner.Scan() {
		currentLine++

		line := scanner.Text()

		indexes := lineHeaderExpr.FindStringIndex(line)
		if indexes == nil {
			return nil, fmt.Errorf("line %d is malformed", currentLine)
		}

		event := line[indexes[1]:]
		nextParser, err := parser.evParser.parseEvent(parser, event)
		if err != nil {
			return nil, fmt.Errorf("failed to parse event: %w", err)
		}

		parser.evParser = nextParser
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	if _, ok := parser.evParser.(*matchParser); ok {
		return nil, errors.New("log entries ended while a match was still open")
	}

	return parser.matches, nil
}

// logParser is an internal type that holds the state of the parsing process.
type logParser struct {
	evParser eventParser
	matches  Matches
}

// newLogParser creates and returns a new instance of logParser.
func newLogParser() *logParser {
	return &logParser{evParser: lookingForGameParser{}}
}

// parseEvent processes an event string with the current event parser, updating the parser's
// state accordingly.
func (p *logParser) parseEvent(event string) error {
	nextParser, err := p.evParser.parseEvent(p, event)
	if err != nil {
		return err
	}

	p.evParser = nextParser
	return nil
}

// eventParser is an interface that defines the methods that must be implemented by the
// different types of parsers. Within the parser, the eventParser is responsible for
// parsing the events and returning the next parser to be used.
type eventParser interface {
	parseEvent(p *logParser, event string) (eventParser, error)
}

// lookingForGameParser is the initial parser that is used to look for the "InitGame" event.
type lookingForGameParser struct{}

// parseEvent checks if the event is the "InitGame" event. If it is, it returns a new
// matchParser, otherwise it returns itself.
func (lfg lookingForGameParser) parseEvent(p *logParser, event string) (eventParser, error) {
	if !strings.HasPrefix(event, "InitGame:") {
		return lfg, nil
	}

	matchParser := newMatchParser()
	return matchParser, nil
}

// matchParser is the parser that is used to parse the events of a match. It keeps track of
// the expected data, and when the "ShutdownGame" event is found, it creates a Match object
// and appends it to the list of matches. After that, it returns to the lookingForGameParser.
type matchParser struct {
	totalKills   int
	players      map[string]struct{}
	kills        map[string]int
	killsByMeans map[string]int
}

// newMatchParser creates and returns a new instance of matchParser.
func newMatchParser() *matchParser {
	return &matchParser{
		players:      make(map[string]struct{}),
		kills:        make(map[string]int),
		killsByMeans: make(map[string]int),
	}
}

// killExpr matches the Kill events. It captures constante elements such as "Kill",
// "killed" and "by" in non capturing groups. The capturing groups output the killer,
// the victim and the means of death.
var killExpr = regexp.MustCompile(`(?:Kill:\s\d+\s\d+\s\d+:\s)(.+)(?:\skilled\s)(.+)(?:\sby\s)([\w]+)`)

func (m *matchParser) parseEvent(p *logParser, event string) (eventParser, error) {
	// this is used instead of ShutdownGame to match the issue at the example log at line
	// 97
	if strings.HasPrefix(event, "---") {
		finishedMatch := Match{
			TotalKills:   m.totalKills,
			Players:      m.getPlayerList(),
			Kills:        m.kills,
			KillsByMeans: m.killsByMeans,
		}
		p.matches = append(p.matches, finishedMatch)
		return lookingForGameParser{}, nil
	}

	matchingGroups := killExpr.FindStringSubmatch(event)
	if len(matchingGroups) == 0 {
		return m, nil
	}

	killer, killed, killedBy := matchingGroups[1], matchingGroups[2], matchingGroups[3]
	m.registerKill(killer, killed, killedBy)

	return m, nil
}

// registerKill registers a kill event in the matchParser's state. It increments the total
// kills, updates the kills count for the killer and the killed player, and increments the
// count for the means of death.
func (m *matchParser) registerKill(killer, killed, killedBy string) {
	m.totalKills++

	for _, player := range [...]string{killer, killed} {
		if player == "<world>" {
			continue
		}

		// this conditional is crucial to make sure even 0 kill players are included
		// in the match info
		if _, ok := m.kills[player]; !ok {
			m.kills[player] = 0
		}
		m.players[player] = struct{}{}
	}

	if killer == "<world>" {
		m.kills[killed]--
	} else if killer != killed {
		m.kills[killer]++
	}

	m.killsByMeans[killedBy]++
}

// getPlayerList returns a slice with the names of the players in the match, sorted alphabetically.
func (m *matchParser) getPlayerList() []string {
	players := make([]string, 0, len(m.players))
	for player := range m.players {
		players = append(players, player)
	}

	slices.Sort(players) // sort the players alphabetically
	return players
}
