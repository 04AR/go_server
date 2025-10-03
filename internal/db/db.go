package db

import (
	"fmt"
	"go-server/internal/auth"
	"log"
	"os"

	"github.com/jmoiron/sqlx"
	_ "modernc.org/sqlite"
)

// InitDB initializes the database and SQLAuthProvider based on environment variables and config file.
func InitDB() (*sqlx.DB, auth.AuthProvider, error) {

	// Load AuthProvider configuration
	configPath := os.Getenv("AUTH_CONFIG_PATH")
	if configPath == "" {
		configPath = "config/config.json"
	}
	config, err := auth.LoadSQLConfigFromFile(configPath)
	if err != nil {
		log.Printf("Failed to load config, using default SQLite config: %v", err)
		config = auth.SQLConfig{
			TableName:        "users",
			IDColumn:         "id",
			UsernameColumn:   "username",
			PasswordColumn:   "password_hash",
			CreateTableSQL:   "CREATE TABLE IF NOT EXISTS users (id INTEGER PRIMARY KEY AUTOINCREMENT, username TEXT UNIQUE NOT NULL, password_hash TEXT NOT NULL)",
			Dialect:          "sqlite",
			ConnectionString: "users.db",
		}
	}

	// Initialize database
	var db *sqlx.DB
	switch config.Dialect {
	case "postgres":
		log.Printf("Initializing Postgres at %v", config.ConnectionString)
		db, err = sqlx.Open("pgx", config.ConnectionString)
	case "mysql":
		log.Printf("Initializing mysql at %v", config.ConnectionString)
		db, err = sqlx.Open("mysql", config.ConnectionString)
	default:
		log.Printf("Initializing sqlite at %v", config.ConnectionString)
		db, err = sqlx.Open("sqlite", config.ConnectionString)
	}
	if err != nil {
		return nil, nil, logError("failed to open database: %v", err)
	}

	// Test database connection
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, nil, logError("failed to ping database: %v", err)
	}

	// Initialize SQLAuthProvider
	authProvider := auth.NewSQLAuthProvider(db, config)
	return db, authProvider, nil
}

// logError logs the error and returns it for handling.
func logError(format string, args ...interface{}) error {
	err := fmt.Errorf(format, args...)
	log.Println(err)
	return err
}
