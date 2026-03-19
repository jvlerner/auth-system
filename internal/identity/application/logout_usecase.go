package application

import (
	"context"
	"fmt"

	"github.com/redis/go-redis/v9"
)

type LogoutCommand struct {
	RefreshToken string `json:"refresh_token"`
}

type LogoutUseCase struct {
	redisClient *redis.Client
}

func NewLogoutUseCase(redisClient *redis.Client) *LogoutUseCase {
	return &LogoutUseCase{
		redisClient: redisClient,
	}
}

func (uc *LogoutUseCase) Execute(ctx context.Context, cmd LogoutCommand) error {
	key := fmt.Sprintf("refresh_token:%s", cmd.RefreshToken)
	
	// Dellete ignora silenciosamente se a key já não existir (idempotente)
	err := uc.redisClient.Del(ctx, key).Err()
	if err != nil {
		return err
	}

	return nil
}
