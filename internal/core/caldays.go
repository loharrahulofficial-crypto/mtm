package core

import "time"

// CalDays returns whole calendar days from from's date to to's date, using
// UTC midnights so it is exact across DST transitions (local midnights can
// be 82800/90000s apart; Hours()/24 then truncates the wrong way).
func CalDays(from, to time.Time) int {
	fd := time.Date(from.Year(), from.Month(), from.Day(), 0, 0, 0, 0, from.Location())
	td := time.Date(to.Year(), to.Month(), to.Day(), 0, 0, 0, 0, to.Location())
	return int(td.UTC().Sub(fd.UTC()).Hours() / 24)
}
