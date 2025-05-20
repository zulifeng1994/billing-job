package models

import (
	"billing-job/config"
	"billing-job/log"
	"context"
	"time"

	"github.com/pkg/errors"

	"github.com/redis/go-redis/v9"
)

// 声明一个全局 Redis 客户端变量
var rc *redis.Client

func init() {
	// 在初始化时创建 Redis 客户端
	rc = redis.NewClient(&redis.Options{
		Addr:     config.Redis.Server,   // Redis 服务器地址
		Password: config.Redis.Password, // Redis 密码
		DB:       0,                     // 使用默认 DB
	})

	// 检查连接
	_, err := rc.Ping(context.Background()).Result()
	if err != nil {
		log.SugarLogger.Errorf("failed to connect to Redis: %v", err)
	}
}

func SaveToRedis(key string, record map[string]string) error {
	if err := rc.HSet(context.Background(), key, record).Err(); err != nil {
		return errors.Wrap(err, "fail to save redis record")
	}
	if err := rc.Expire(context.Background(), key, 24*time.Hour).Err(); err != nil {
		return errors.Wrap(err, "fail to set expiration for redis record")
	}
	return nil
}

func RedisGetByKey(key string) (map[string]string, error) {
	result, err := rc.HGetAll(context.Background(), key).Result()
	if err != nil {
		return nil, errors.Wrap(err, "failed to get record from Redis")
	}
	return result, nil
}

func GetKeyNames(key string) (interface{}, error) {
	return rc.Keys(context.Background(), key).Result()
}

func RedisDel(key string) error {
	return rc.Del(context.Background(), key).Err()
}

func RedisSet(key string, value map[string]string) (bool, error) {
	return rc.HMSet(context.Background(), key, value).Result()
}

const (
	lockKeyPrefix = "billing:lock:"
	lockTimeout   = 60 * time.Minute
	//lockRetryTime = 5 * time.Second
)

type RedisLock struct {
	client     *redis.Client
	key        string
	value      string
	ctx        context.Context
	cancelFunc context.CancelFunc
}
