package db

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/fsnotify/fsnotify"
	"github.com/redis/go-redis/v9"
)

type RedisManager struct {
	Client  *redis.Client
	PubSub  *redis.PubSub
	scripts map[string]string // action -> sha1
	mu      sync.RWMutex
}

func InitRedis(addr string, password string, scriptDir string) (*RedisManager, error) {
	rdb := redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: password,
	})

	// Test connection
	if err := rdb.Ping(context.Background()).Err(); err != nil {
		return nil, fmt.Errorf("ping redis: %w", err)
	}

	db := &RedisManager{
		Client:  rdb,
		scripts: make(map[string]string),
	}

	ctx := context.Background()

	// load existing scripts
	if err := db.loadScriptsFromDir(ctx, scriptDir); err != nil {
		return nil, err
	}

	// watch for changes
	if err := db.watchDir(ctx, scriptDir); err != nil {
		return nil, err
	}

	log.Printf("Redis initialized. Connected to %s, scripts from %s", addr, scriptDir)
	return db, nil
}

func (db *RedisManager) loadScriptsFromDir(ctx context.Context, dir string) error {
	files, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("read dir: %w", err)
	}
	for _, file := range files {
		if file.IsDir() || !strings.HasSuffix(file.Name(), ".lua") {
			continue
		}
		if err := db.loadScriptFile(ctx, filepath.Join(dir, file.Name())); err != nil {
			log.Printf("failed to load %s: %v", file.Name(), err)
		}
	}
	return nil
}

func (db *RedisManager) loadScriptFile(ctx context.Context, path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	sha, err := db.Client.ScriptLoad(ctx, string(data)).Result()
	if err != nil {
		return err
	}
	name := strings.TrimSuffix(filepath.Base(path), ".lua")

	db.mu.Lock()
	db.scripts[name] = sha
	db.mu.Unlock()

	log.Printf("Loaded script %s (%s)", name, sha)
	return nil
}

func (db *RedisManager) unloadScriptFile(path string) {
	name := strings.TrimSuffix(filepath.Base(path), ".lua")

	db.mu.Lock()
	delete(db.scripts, name)
	db.mu.Unlock()

	log.Printf("Unloaded script %s", name)
}

func (db *RedisManager) watchDir(ctx context.Context, dir string) error {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}
	if err := watcher.Add(dir); err != nil {
		return err
	}

	go func() {
		for {
			select {
			case event := <-watcher.Events:
				if strings.HasSuffix(event.Name, ".lua") {
					if event.Op&(fsnotify.Create|fsnotify.Write) != 0 {
						if err := db.loadScriptFile(ctx, event.Name); err != nil {
							log.Printf("reload failed for %s: %v", event.Name, err)
						}
					}
					if event.Op&(fsnotify.Remove|fsnotify.Rename) != 0 {
						db.unloadScriptFile(event.Name)
					}
				}
			case err := <-watcher.Errors:
				log.Println("watcher error:", err)
			case <-ctx.Done():
				watcher.Close()
				return
			}
		}
	}()
	return nil
}

func (db *RedisManager) CallScript(ctx context.Context, action string, keys []string, args ...interface{}) (map[string]interface{}, error) {
	db.mu.RLock()
	sha, ok := db.scripts[action]
	db.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("script %s not loaded", action)
	}

	res, err := db.Client.EvalSha(ctx, sha, keys, args...).Result()
	if err != nil {
		return nil, err
	}

	// Expect Lua scripts to return JSON-compatible maps/tables
	switch val := res.(type) {
	case map[string]interface{}:
		return val, nil
	case []interface{}:
		// If Lua returned array, wrap into result
		return map[string]interface{}{"result": val}, nil
	case string:
		return map[string]interface{}{"result": val}, nil
	default:
		return map[string]interface{}{"result": val}, nil
	}
}

func (db *RedisManager) Shutdown(ctx context.Context, wipeData bool) {
	log.Println("Shutting down Redis...")

	// Unload all Lua scripts
	if err := db.Client.ScriptFlush(ctx).Err(); err != nil {
		log.Printf("failed to flush scripts: %v", err)
	} else {
		log.Println("All Lua scripts flushed")
	}

	// Wipe data if requested
	if wipeData {
		if err := db.Client.FlushDB(ctx).Err(); err != nil {
			log.Printf("failed to flush DB: %v", err)
		} else {
			log.Println("All Redis data cleared")
		}
	}

	// Close client
	if err := db.Client.Close(); err != nil {
		log.Printf("failed to close Redis client: %v", err)
	} else {
		log.Println("Redis client closed")
	}
}
