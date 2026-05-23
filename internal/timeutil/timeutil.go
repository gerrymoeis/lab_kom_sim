package timeutil

import (
	"log"
	"sync"
	"time"
)

var (
	loc     *time.Location
	locOnce sync.Once
)

func SetTimezone(tz string) {
	locOnce.Do(func() {
		if tz == "" || tz == "Local" {
			loc = time.Local
			return
		}
		if l, err := time.LoadLocation(tz); err == nil {
			loc = l
			return
		}
		log.Printf("timeutil: invalid timezone %q, falling back to Local", tz)
		loc = time.Local
	})
}

func Location() *time.Location {
	locOnce.Do(func() { loc = time.Local })
	return loc
}

func FromDB(t time.Time) time.Time {
	if t.IsZero() {
		return t
	}
	return t.In(Location())
}

func Code() string {
	name, _ := time.Now().In(Location()).Zone()
	return name
}
