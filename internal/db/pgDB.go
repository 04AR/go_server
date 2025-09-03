package DB

import (
	"log"

	"database/sql"

	"golang.org/x/crypto/bcrypt"
)

func InitPG(db *sql.DB) {
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS users (
			id SERIAL PRIMARY KEY,
			username TEXT UNIQUE NOT NULL,
			password_hash TEXT NOT NULL
		);
	`)
	if err != nil {
		log.Fatal("Failed to init SQLite:", err)
	}
}

func TestDataPG(db *sql.DB) {
	// Insert test user (username: test, password: password)
	hash, err := bcrypt.GenerateFromPassword([]byte("password"), bcrypt.DefaultCost)
	if err != nil {
		log.Fatal("Failed to hash password:", err)
	}
	_, err = db.Exec("INSERT INTO users (username, password_hash) VALUES ($1, $2) ON CONFLICT (username) DO NOTHING;", "test", hash)
	if err != nil {
		log.Fatal("Failed to insert test user:", err)
	}

}
