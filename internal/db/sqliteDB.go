package db

import (
	"log"

	"github.com/jmoiron/sqlx"
	"golang.org/x/crypto/bcrypt"
)

func InitSqlite(db *sqlx.DB) {
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS users (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			username TEXT UNIQUE NOT NULL,
			password_hash TEXT NOT NULL
		);
	`)
	if err != nil {
		log.Fatal("Failed to init SQLite:", err)
	}
}

func TestDataSqlite(db *sqlx.DB) {
	// Insert test user (username: test, password: password)
	hash, err := bcrypt.GenerateFromPassword([]byte("password"), bcrypt.DefaultCost)
	if err != nil {
		log.Fatal("Failed to hash password:", err)
	}
	_, err = db.Exec("INSERT OR IGNORE INTO users (username, password_hash) VALUES (?, ?)", "test", hash)
	if err != nil {
		log.Fatal("Failed to insert test user:", err)
	}

}
