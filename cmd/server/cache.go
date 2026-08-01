package main

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

// Cache-aside layer for shortcode -> URL lookups. A nil rdb means no cache
// is configured (or Redis was unreachable at startup), and every method
// becomes a no-op so the app runs fine without one.
type Cache struct {
	rdb *redis.Client
}

const urlCacheTTL = 24 * time.Hour

func NewCache(redisURL string) *Cache {
	if redisURL == "" {
		log.Println("REDIS_URL not set, running without a cache layer")
		return &Cache{}
	}

	opts, err := redis.ParseURL(redisURL)
	if err != nil {
		log.Printf("Invalid REDIS_URL, running without a cache layer: %v", err)
		return &Cache{}
	}

	rdb := redis.NewClient(opts)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := rdb.Ping(ctx).Err(); err != nil {
		log.Printf("Could not connect to Redis, running without a cache layer: %v", err)
		return &Cache{}
	}

	return &Cache{rdb: rdb}
}

func (c *Cache) Close() {
	if c.rdb != nil {
		c.rdb.Close()
	}
}

// GetURL looks up a shortcode. ok is false on a miss or if caching is disabled.
func (c *Cache) GetURL(ctx context.Context, shortCode string) (id int64, originalURL string, ok bool) {
	if c.rdb == nil {
		return 0, "", false
	}

	val, err := c.rdb.Get(ctx, "url:"+shortCode).Result()
	if err != nil {
		return 0, "", false
	}

	idStr, url, found := strings.Cut(val, "\n")
	id, err = strconv.ParseInt(idStr, 10, 64)
	if !found || err != nil {
		return 0, "", false
	}

	return id, url, true
}

func (c *Cache) SetURL(ctx context.Context, shortCode string, id int64, originalURL string) {
	if c.rdb == nil {
		return
	}

	val := fmt.Sprintf("%d\n%s", id, originalURL)
	if err := c.rdb.Set(ctx, "url:"+shortCode, val, urlCacheTTL).Err(); err != nil {
		log.Println("Error writing to cache:", err)
	}
}
