package starshipkepler

import "time"

type ScoreEntry struct {
	Name  string
	Score int
	Time  time.Time
}

func (data *LocalData) Highscore() ScoreEntry {
	highscore := ScoreEntry{}

	for _, scoreEntry := range data.Scoreboard {
		if scoreEntry.Score > highscore.Score {
			highscore = scoreEntry
		}
	}

	return highscore
}

func (data *LocalData) NewScore(score ScoreEntry) {
	data.Scoreboard = append(data.Scoreboard, score)
}
