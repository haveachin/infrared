package session_validator

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"time"

	"github.com/cespare/xxhash/v2"
	"github.com/go-redis/redis/v9"
	"github.com/gofrs/uuid"
)

var (
	errValidationNotFound = errors.New("validation not found")
)

type storage interface {
	GetValidation(username string, ip net.IP) (uuid.UUID, error)
	PutValidation(username string, ip net.IP, uuid uuid.UUID) error
}

type redisConfig struct {
	URI string        `mapstrucutre:"uri"`
	TTL time.Duration `mapstrucutre:"ttl"`
}

type redisStorage struct {
	cli          *redis.Client
	readTimeout  time.Duration
	writeTimeout time.Duration
	ttl          time.Duration
}

func newRedis(cfg redisConfig) (*redisStorage, error) {
	opts, err := redis.ParseURL(cfg.URI)
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), opts.ReadTimeout)
	defer cancel()

	cli := redis.NewClient(opts)
	if err := cli.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to ping redis; %v", err)
	}

	return &redisStorage{
		cli:          cli,
		readTimeout:  opts.ReadTimeout,
		writeTimeout: opts.WriteTimeout,
		ttl:          cfg.TTL,
	}, nil
}

func hashUsernameAndIP(username string, ip net.IP) string {
	key := xxhash.New()
	key.WriteString(username)
	key.Write(ip)
	// preallowcate 8 bytes for uint64 hash
	sum := key.Sum(make([]byte, 0, 8))
	return hex.EncodeToString(sum)
}

func (s redisStorage) PutValidation(username string, ip net.IP, uuid uuid.UUID) error {
	key := hashUsernameAndIP(username, ip)
	ctx, cancel := context.WithTimeout(context.Background(), s.readTimeout)
	defer cancel()
	return s.cli.Set(ctx, key, uuid.Bytes(), s.ttl).Err()
}

func (s redisStorage) GetValidation(username string, ip net.IP) (uuid.UUID, error) {
	key := hashUsernameAndIP(username, ip)
	ctx, cancel := context.WithTimeout(context.Background(), s.readTimeout)
	defer cancel()
	v, err := s.cli.Get(ctx, key).Result()
	if err != nil {
		if err == redis.Nil {
			return uuid.Nil, errValidationNotFound
		}
		return uuid.Nil, err
	}
	return uuid.FromBytes([]byte(v))
}
