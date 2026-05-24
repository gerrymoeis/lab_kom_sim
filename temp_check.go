package main

import (
	"database/sql"
	"fmt"
	_ "github.com/mattn/go-sqlite3"
)

func main() {
	db, err := sql.Open("sqlite3", "inventaris_lab.db")
	if err != nil { fmt.Println(err); return }
	defer db.Close()
	rows, _ := db.Query("SELECT name FROM sqlite_master WHERE type='table' ORDER BY name")
	for rows.Next() { var n string; rows.Scan(&n); fmt.Println(n) }
	if err := rows.Err(); err != nil { fmt.Println("rows err:", err) }
}
