package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/jmoiron/sqlx"
	"golang.org/x/crypto/bcrypt"
)

// SQLConfig defines the schema and queries for SQLAuthProvider.
type SQLConfig struct {
	TableName        string `json:"table_name"`
	IDColumn         string `json:"id_column"`
	UsernameColumn   string `json:"username_column"`
	PasswordColumn   string `json:"password_column"`
	CreateTableSQL   string `json:"create_table_sql"`
	Dialect          string `json:"dialect"` // "sqlite", "postgres", "mysql"
	ConnectionString string `json:"connection_string"`
}

// Validate checks if the configuration is valid.
func (c SQLConfig) Validate() error {
	if c.TableName == "" || c.IDColumn == "" || c.UsernameColumn == "" || c.PasswordColumn == "" {
		return fmt.Errorf("table_name, id_column, username_column, and password_column are required")
	}
	if c.Dialect != "" && c.Dialect != "sqlite" && c.Dialect != "postgres" && c.Dialect != "mysql" {
		return fmt.Errorf("invalid dialect: %s (must be sqlite, postgres, or mysql)", c.Dialect)
	}
	if c.CreateTableSQL == "" {
		return fmt.Errorf("create_table_sql is required")
	}
	if c.ConnectionString == "" && c.Dialect != "sqlite" {
		return fmt.Errorf("connection_string is required for dialect %s", c.Dialect)
	}
	return nil
}

// SQLAuthProvider implements AuthProvider for any SQL database with a configurable schema.
type SQLAuthProvider struct {
	db     *sqlx.DB
	config SQLConfig
}

// NewSQLAuthProvider creates a new SQLAuthProvider with the given config.
func NewSQLAuthProvider(db *sqlx.DB, config SQLConfig) *SQLAuthProvider {
	// Validate configuration
	if err := config.Validate(); err != nil {
		panic("Invalid SQL config: " + err.Error())
	}

	// Initialize table
	_, err := db.Exec(config.CreateTableSQL)
	if err != nil {
		panic("Failed to init table: " + err.Error())
	}
	return &SQLAuthProvider{db: db, config: config}
}

// LoadSQLConfigFromFile loads SQLConfig from a JSON file.
func LoadSQLConfigFromFile(path string) (SQLConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return SQLConfig{}, fmt.Errorf("failed to read config file: %v", err)
	}
	var config SQLConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return SQLConfig{}, fmt.Errorf("failed to parse config: %v", err)
	}
	return config, nil
}

func (s *SQLAuthProvider) Register(ctx context.Context, username, password string) (User, error) {
	username = strings.TrimSpace(username)
	password = strings.TrimSpace(password)
	if username == "" || password == "" {
		return User{}, fmt.Errorf("username and password are required")
	}
	if len(username) < 3 || len(password) < 6 {
		return User{}, fmt.Errorf("username must be at least 3 characters and password at least 6 characters")
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return User{}, fmt.Errorf("failed to hash password: %v", err)
	}

	var id int
	if s.config.Dialect == "postgres" {
		// PostgreSQL: Use RETURNING clause to get ID
		query := fmt.Sprintf("INSERT INTO %s (%s, %s) VALUES ($1, $2) RETURNING %s", s.config.TableName, s.config.UsernameColumn, s.config.PasswordColumn, s.config.IDColumn)
		err = s.db.QueryRowContext(ctx, query, username, hash).Scan(&id)
	} else {
		// SQLite, MySQL: Use LastInsertId
		query := fmt.Sprintf("INSERT INTO %s (%s, %s) VALUES (?, ?)", s.config.TableName, s.config.UsernameColumn, s.config.PasswordColumn)
		res, err := s.db.ExecContext(ctx, query, username, hash)
		if err != nil {
			if strings.Contains(err.Error(), "UNIQUE constraint failed") || strings.Contains(err.Error(), "Duplicate entry") || strings.Contains(err.Error(), "unique") {
				return User{}, fmt.Errorf("username already taken")
			}
			return User{}, fmt.Errorf("failed to insert user: %v", err)
		}
		id64, err := res.LastInsertId()
		if err != nil {
			return User{}, fmt.Errorf("failed to get user ID: %v", err)
		}
		id = int(id64)
	}

	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint failed") || strings.Contains(err.Error(), "Duplicate entry") || strings.Contains(err.Error(), "unique") {
			return User{}, fmt.Errorf("username already taken")
		}
		return User{}, fmt.Errorf("failed to insert user: %v", err)
	}

	return User{ID: id, Username: username, IsGuest: false}, nil
}

func (s *SQLAuthProvider) Login(ctx context.Context, username, password string) (User, error) {
	var id int
	var hash string
	var query string
	if s.config.Dialect == "postgres" {
		// PostgreSQL: Case-sensitive comparison
		query = fmt.Sprintf("SELECT %s, %s FROM %s WHERE %s = $1", s.config.IDColumn, s.config.PasswordColumn, s.config.TableName, s.config.UsernameColumn)
		err := s.db.QueryRowContext(ctx, query, username).Scan(&id, &hash)
		if err != nil {
			return User{}, fmt.Errorf("invalid credentials")
		}
	} else {
		log.Println("sqlite login")
		// SQLite, MySQL: Case-insensitive comparison
		query = fmt.Sprintf("SELECT %s, %s FROM %s WHERE LOWER(%s) = LOWER(?)", s.config.IDColumn, s.config.PasswordColumn, s.config.TableName, s.config.UsernameColumn)
		err := s.db.QueryRowContext(ctx, query, username).Scan(&id, &hash)
		if err != nil {
			log.Println("sqlite login error", err)
			return User{}, fmt.Errorf("invalid credentials")
		}
	}

	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)); err != nil {
		return User{}, fmt.Errorf("invalid credentials")
	}
	return User{ID: id, Username: username, IsGuest: false}, nil
}

func (s *SQLAuthProvider) ValidateUser(ctx context.Context, userID int) (bool, error) {
	var exists bool
	var query string
	if s.config.Dialect == "postgres" {
		// PostgreSQL: Use $1 placeholder
		query = fmt.Sprintf("SELECT EXISTS(SELECT 1 FROM %s WHERE %s = $1)", s.config.TableName, s.config.IDColumn)
	} else {
		log.Println("sqlite validate")
		// SQLite, MySQL: Use ? placeholder
		query = fmt.Sprintf("SELECT EXISTS(SELECT 1 FROM %s WHERE %s = ?)", s.config.TableName, s.config.IDColumn)
	}
	err := s.db.QueryRowContext(ctx, query, userID).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("failed to validate user: %v", err)
	}
	return exists, nil
}

func (s *SQLAuthProvider) GetUser(ctx context.Context, userID int) (User, error) {
	var username string
	query := s.buildQuery("SELECT %s FROM %s WHERE %s = ?")
	err := s.db.QueryRowContext(ctx, query, userID).Scan(&username)
	if err != nil {
		return User{}, fmt.Errorf("failed to get user: %v", err)
	}
	return User{ID: userID, Username: username, IsGuest: false}, nil
}

// buildQuery constructs a query with the correct placeholder syntax based on dialect.
func (s *SQLAuthProvider) buildQuery(template string) string {
	switch s.config.Dialect {
	case "postgres":
		query := template
		query = strings.ReplaceAll(query, "?", "$1")
		for i := 2; strings.Contains(query, "?"); i++ {
			query = strings.Replace(query, "?", fmt.Sprintf("$%d", i), 1)
		}
		return fmt.Sprintf(query, s.config.TableName, s.config.IDColumn, s.config.PasswordColumn, s.config.UsernameColumn)
	default: // sqlite, mysql
		return fmt.Sprintf(template, s.config.TableName, s.config.IDColumn, s.config.PasswordColumn, s.config.UsernameColumn)
	}
}
