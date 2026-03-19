package infrastructure

import (
	"context"
	"encoding/json"
	"time"

	"github.com/jvlerner/auth-system/internal/identity/application"
	"github.com/redis/go-redis/v9"
)

type RedisProfileCache struct {
	client *redis.Client
}

func NewRedisProfileCache(client *redis.Client) application.ProfileCache {
	return &RedisProfileCache{
		client: client,
	}
}

func (c *RedisProfileCache) GetProfile(ctx context.Context, id string) (*application.UserProfileDTO, error) {
	val, err := c.client.Get(ctx, "user:profile:"+id).Result()
	if err != nil {
		if err == redis.Nil {
			return nil, nil // cache miss
		}
		return nil, err
	}

	var profile application.UserProfileDTO
	if err := json.Unmarshal([]byte(val), &profile); err != nil {
		return nil, err
	}

	return &profile, nil
}

func (c *RedisProfileCache) SetProfile(ctx context.Context, profile *application.UserProfileDTO) error {
	data, err := json.Marshal(profile)
	if err != nil {
		return err
	}
	// Expira em 10 minutos
	return c.client.Set(ctx, "user:profile:"+profile.ID, data, 10*time.Minute).Err()
}
