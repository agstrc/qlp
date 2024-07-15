package qlp

import (
	"bytes"
	_ "embed"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

//go:embed test_log.txt
var testLogFile []byte

func TestEventParsingInitGame(t *testing.T) {
	p := newLogParser()
	err := p.parseEvent("InitGame:")
	assert.NoError(t, err)
	assert.IsType(t, &matchParser{}, p.evParser)
}

func TestEventParsingShutdownGame(t *testing.T) {
	p := newLogParser()
	err := p.parseEvent("InitGame:")
	assert.NoError(t, err)
	err = p.parseEvent("ShutdownGame:")
	assert.NoError(t, err)
	assert.IsType(t, lookingForGameParser{}, p.evParser)
	assert.Len(t, p.matches, 1)
}

func TestKillEventHandling(t *testing.T) {
	p := newLogParser()
	p.parseEvent("InitGame:")
	p.parseEvent("Kill: 0 1 2: Isgalamido killed Mocinha by MOD_ROCKET")
	p.parseEvent("ShutdownGame:")
	match := p.matches[0]
	assert.Equal(t, 1, match.Kills["Isgalamido"])
	assert.Contains(t, match.Players, "Isgalamido")
	assert.Contains(t, match.Players, "Mocinha")
}

func TestKillEventWithWorldKiller(t *testing.T) {
	p := newLogParser()
	p.parseEvent("InitGame:")
	p.parseEvent("Kill: 0 1 2: <world> killed Mocinha by MOD_ROCKET")
	p.parseEvent("ShutdownGame:")
	match := p.matches[0]
	assert.Equal(t, -1, match.Kills["Mocinha"])
}

func TestMultipleMatches(t *testing.T) {
	p := logParser{evParser: lookingForGameParser{}}
	p.parseEvent("InitGame:")
	p.parseEvent("Kill: 0 1 2: Isgalamido killed Mocinha by MOD_ROCKET")
	p.parseEvent("ShutdownGame:")
	p.parseEvent("InitGame:")
	p.parseEvent("Kill: 0 1 2: Mocinha killed Isgalamido by MOD_ROCKET")
	p.parseEvent("ShutdownGame:")
	assert.Len(t, p.matches, 2)
	assert.Equal(t, 1, p.matches[0].Kills["Isgalamido"])
	assert.Equal(t, 1, p.matches[1].Kills["Mocinha"])
}

func TestMalformedLineHandling(t *testing.T) {
	log := "InitGame:\nKill: 0 1 2: Isgalamido killed Mocinha by MOD_ROCKET\nBadLine\nShutdownGame:"
	_, err := ParseLog(strings.NewReader(log))
	assert.Error(t, err)
	assert.ErrorContains(t, err, "is malformed")
}

func TestKillByMeansCounting(t *testing.T) {
	p := logParser{evParser: lookingForGameParser{}}
	p.parseEvent("InitGame:")
	p.parseEvent("Kill: 0 1 2: Isgalamido killed Mocinha by MOD_ROCKET")
	p.parseEvent("Kill: 0 1 2: <world> killed Mocinha by MOD_TRIGGER_HURT")
	p.parseEvent("ShutdownGame:")
	match := p.matches[0]
	assert.Equal(t, 1, match.KillsByMeans["MOD_ROCKET"])
	assert.Equal(t, 1, match.KillsByMeans["MOD_TRIGGER_HURT"])
}

func TestLogEntriesEndedWhileMatchStillOpen(t *testing.T) {
	log := "  0:00 ------------------------------------------------------------\n  0:00 InitGame:"
	_, err := ParseLog(strings.NewReader(log))
	assert.Error(t, err)
	assert.ErrorContains(t, err, "log entries ended while a match was still open")
}

func TestTotalKillsCounting(t *testing.T) {
	p := newLogParser()
	p.parseEvent("InitGame:")
	p.parseEvent("Kill: 0 1 2: Isgalamido killed Mocinha by MOD_ROCKET")
	p.parseEvent("Kill: 0 1 2: <world> killed Isgalamido by MOD_TRIGGER_HURT")
	p.parseEvent("ShutdownGame:")
	match := p.matches[0]
	assert.Equal(t, 2, match.TotalKills, "Total kills should count all kills, including world kills")
}

func TestParseLog(t *testing.T) {
	matches, err := ParseLog(bytes.NewReader(testLogFile))
	assert.NoError(t, err)

	assert.Len(t, matches, 21)

	firstMatch := matches[0]
	assert.Equal(t, 0, len(firstMatch.Kills))
	assert.Equal(t, 0, firstMatch.TotalKills)

	secondMatch := matches[1]
	assert.Equal(t, 11, secondMatch.TotalKills)
	assert.Equal(t, 2, len(secondMatch.Kills))
	assert.Equal(t, 2, len(secondMatch.Players))
	assert.Equal(t, -5, secondMatch.Kills["Isgalamido"])

	thirdMatch := matches[2]
	assert.Equal(t, 4, thirdMatch.TotalKills)
	assert.Equal(
		t,
		[]string{"Dono da Bola", "Isgalamido", "Mocinha", "Zeh"},
		thirdMatch.Players,
	)
	assert.Equal(
		t,
		map[string]int{"Dono da Bola": -1, "Isgalamido": 1, "Mocinha": 0, "Zeh": -2},
		thirdMatch.Kills,
	)
	assert.Equal(
		t,
		map[string]int{"MOD_ROCKET": 1, "MOD_TRIGGER_HURT": 2, "MOD_FALLING": 1},
		thirdMatch.KillsByMeans,
	)
}
