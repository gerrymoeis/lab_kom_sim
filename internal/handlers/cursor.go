package handlers

import (
	"encoding/base64"
	"strconv"
	"strings"
	"time"

	"inventaris-lab-kom/internal/models"
)

func encodeCursor(vals ...string) string {
	return base64.RawURLEncoding.EncodeToString([]byte(strings.Join(vals, "|")))
}

func decodeCursor(s string) ([]string, bool) {
	b, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		return nil, false
	}
	return strings.Split(string(b), "|"), true
}

func parseLogbookCursor(cursor string) (id int64, date, timeIn string) {
	parts, ok := decodeCursor(cursor)
	if !ok || len(parts) < 3 {
		return 0, "", ""
	}
	id, _ = strconv.ParseInt(parts[2], 10, 64)
	return id, parts[0], parts[1]
}

func buildLogbookCursor(e models.LogbookEntry) string {
	return encodeCursor(e.Date.Format("2006-01-02"), e.TimeIn, strconv.Itoa(e.ID))
}

func parseActivityCursor(cursor string) (id int64, createdAt time.Time) {
	parts, ok := decodeCursor(cursor)
	if !ok || len(parts) < 2 {
		return 0, time.Time{}
	}
	id, _ = strconv.ParseInt(parts[1], 10, 64)
	createdAt, _ = time.Parse(time.RFC3339Nano, parts[0])
	return id, createdAt
}

func buildActivityCursor(l models.ActivityLog) string {
	return encodeCursor(l.CreatedAt.Format(time.RFC3339Nano), strconv.Itoa(l.ID))
}
