package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/redis/go-redis/v9"
)

var jwtSecret []byte

func init() {
	secret := os.Getenv("JWT_SECRET")
	if secret == "" {
		secret = "jwt_secret"
		// log.Fatal("JWT_SECRET environment variable not set")
	}
	jwtSecret = []byte(secret)
}

type registerReq struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type loginReq struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func RegisterHandler(authProvider AuthProvider) http.HandlerFunc {
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

		user, err := authProvider.Register(r.Context(), req.Username, req.Password)
		if err != nil {
			if err.Error() == "username already taken" {
				http.Error(w, err.Error(), http.StatusConflict)
			} else if strings.Contains(err.Error(), "required") || strings.Contains(err.Error(), "at least") {
				http.Error(w, err.Error(), http.StatusBadRequest)
			} else {
				http.Error(w, "Internal error", http.StatusInternalServerError)
			}
			return
		}

		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"message": "User created successfully",
			"user":    user,
		})
	}
}

func LoginHandler(authProvider AuthProvider) http.HandlerFunc {
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

		user, err := authProvider.Login(r.Context(), req.Username, req.Password)
		if err != nil {
			http.Error(w, "Invalid credentials", http.StatusUnauthorized)
			return
		}

		claims := jwt.MapClaims{
			"user": map[string]interface{}{
				"id":       user.ID,
				"username": user.Username,
				"is_guest": user.IsGuest,
			},
			"exp": time.Now().Add(24 * time.Hour).Unix(),
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

		ctx := r.Context()
		guestID, err := redisClient.Incr(ctx, "guest_id_counter").Result()
		if err != nil {
			http.Error(w, "Internal error", http.StatusInternalServerError)
			return
		}
		guestID = -guestID // Negative IDs for guests

		guestKey := fmt.Sprintf("guest:%d", guestID)
		err = redisClient.HSet(ctx, guestKey, map[string]interface{}{
			"username":   fmt.Sprintf("Guest_%d", -guestID),
			"created_at": time.Now().Unix(),
		}).Err()
		if err != nil {
			http.Error(w, "Internal error", http.StatusInternalServerError)
			return
		}
		redisClient.Expire(ctx, guestKey, 1*time.Hour)

		user := User{
			ID:       int(guestID),
			Username: fmt.Sprintf("Guest_%d", -guestID),
			IsGuest:  true,
		}

		claims := jwt.MapClaims{
			"user": map[string]interface{}{
				"id":       user.ID,
				"username": user.Username,
				"is_guest": user.IsGuest,
			},
			"exp": time.Now().Add(1 * time.Hour).Unix(),
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

func ValidateJWT(tokenStr string, authProvider AuthProvider, redisClient *redis.Client) (User, error) {
	token, err := jwt.Parse(tokenStr, func(token *jwt.Token) (interface{}, error) {
		return jwtSecret, nil
	})
	if err != nil || !token.Valid {
		return User{}, fmt.Errorf("invalid token: %v", err)
	}
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return User{}, jwt.ErrInvalidKey
	}

	userClaims, ok := claims["user"].(map[string]interface{})
	if !ok {
		return User{}, fmt.Errorf("invalid user claims")
	}
	userID, ok := userClaims["id"].(float64)
	if !ok {
		return User{}, fmt.Errorf("invalid user ID")
	}
	username, _ := userClaims["username"].(string) // Optional, may be empty
	isGuest, _ := userClaims["is_guest"].(bool)

	user := User{
		ID:       int(userID),
		Username: username,
		IsGuest:  isGuest,
	}

	if isGuest {
		guestKey := fmt.Sprintf("guest:%d", user.ID)
		exists, err := redisClient.Exists(context.Background(), guestKey).Result()
		if err != nil || exists == 0 {
			return User{}, fmt.Errorf("guest user not found")
		}
		// Update username from Redis if not in JWT
		if user.Username == "" {
			user.Username, _ = redisClient.HGet(context.Background(), guestKey, "username").Result()
		}
	} else {
		exists, err := authProvider.ValidateUser(context.Background(), user.ID)
		if err != nil || !exists {
			return User{}, fmt.Errorf("registered user not found")
		}
		// Update username from AuthProvider if not in JWT
		if user.Username == "" {
			updatedUser, err := authProvider.GetUser(context.Background(), user.ID)
			if err != nil {
				return User{}, fmt.Errorf("failed to get user: %v", err)
			}
			user.Username = updatedUser.Username
		}
	}
	return user, nil
}
