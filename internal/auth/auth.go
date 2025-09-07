package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/jmoiron/sqlx"
	"github.com/redis/go-redis/v9"
	"golang.org/x/crypto/bcrypt"
)

var jwtSecret []byte = []byte("your-very-secret-key") // CHANGE THIS in production

func init() {

}

type registerReq struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type loginReq struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func RegisterHandler(db *sqlx.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var req registerReq
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request", http.StatusBadRequest)
			return
		}

		// Validate input
		req.Username = strings.TrimSpace(req.Username)
		req.Password = strings.TrimSpace(req.Password)
		if req.Username == "" || req.Password == "" {
			http.Error(w, "Username and password are required", http.StatusBadRequest)
			return
		}
		if len(req.Username) < 3 || len(req.Password) < 6 {
			http.Error(w, "Username must be at least 3 characters and password at least 6 characters", http.StatusBadRequest)
			return
		}

		// Hash password
		hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
		if err != nil {
			http.Error(w, "Internal error", http.StatusInternalServerError)
			return
		}

		// Insert user
		_, err = db.Exec("INSERT INTO users (username, password_hash) VALUES (?, ?)", req.Username, hash)
		if err != nil {
			if strings.Contains(err.Error(), "UNIQUE constraint failed") {
				http.Error(w, "Username already taken", http.StatusConflict)
				return
			}
			http.Error(w, "Internal error", http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]string{"message": "User created successfully"})
	}
}

func LoginHandler(db *sqlx.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var req loginReq
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request", http.StatusBadRequest)
			return
		}

		var id int
		var hash string
		err := db.QueryRow("SELECT id, password_hash FROM users WHERE username = ?", req.Username).Scan(&id, &hash)
		if err != nil {
			http.Error(w, "Invalid credentials", http.StatusUnauthorized)
			return
		}

		if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(req.Password)); err != nil {
			http.Error(w, "Invalid credentials", http.StatusUnauthorized)
			return
		}

		claims := jwt.MapClaims{
			"user_id": id,
			"exp":     time.Now().Add(24 * time.Hour).Unix(),
		}
		token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
		signed, err := token.SignedString(jwtSecret)
		if err != nil {
			http.Error(w, "Internal error", http.StatusInternalServerError)
			return
		}

		json.NewEncoder(w).Encode(map[string]string{"token": signed})
	}
}

func GuestHandler(redisClient *redis.Client) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		// Generate unique guest ID (negative to avoid collision with SQLite IDs)
		ctx := context.Background()
		guestID, err := redisClient.Incr(ctx, "guest_id_counter").Result()
		if err != nil {
			http.Error(w, "Internal error", http.StatusInternalServerError)
			return
		}
		guestID = -guestID // Negative IDs for guests

		// Store guest metadata in Redis (expires after 1 hour)
		guestKey := "guest:" + strconv.FormatInt(guestID, 10)
		err = redisClient.HSet(ctx, guestKey, map[string]interface{}{
			"created_at": time.Now().Unix(),
		}).Err()
		if err != nil {
			http.Error(w, "Internal error", http.StatusInternalServerError)
			return
		}
		redisClient.Expire(ctx, guestKey, 1*time.Hour)

		// Generate short-lived JWT (1 hour)
		claims := jwt.MapClaims{
			"user_id":  guestID,
			"is_guest": true,
			"exp":      time.Now().Add(1 * time.Hour).Unix(),
		}
		token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
		signed, err := token.SignedString(jwtSecret)
		if err != nil {
			http.Error(w, "Internal error", http.StatusInternalServerError)
			return
		}

		json.NewEncoder(w).Encode(map[string]string{"token": signed})
	}
}

func ValidateJWT(tokenStr string, db *sqlx.DB) (int, bool, error) {
	token, err := jwt.Parse(tokenStr, func(token *jwt.Token) (interface{}, error) {
		return jwtSecret, nil
	})
	if err != nil || !token.Valid {
		return 0, false, err
	}
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return 0, false, jwt.ErrInvalidKey
	}
	userID := int(claims["user_id"].(float64))

	// If guest flag is set in token → no DB check
	if guest, ok := claims["guest"].(bool); ok && guest {
		return userID, true, nil
	}

	// Check if user is in the database (for registered users)
	var exists bool
	err = db.QueryRow("SELECT EXISTS(SELECT 1 FROM users WHERE id = ?)", userID).Scan(&exists)
	if err != nil || !exists {
		return 0, false, fmt.Errorf("registered user not found")
	}

	return userID, false, nil
}
