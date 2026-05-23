package timeutil

import (
	"time"
)

func FromDB(t time.Time) time.Time {
	if t.IsZero() {
		return t
	}
	return t.In(time.Local)
}

func Code() string {
	name, _ := time.Now().Zone()
	return name
}
