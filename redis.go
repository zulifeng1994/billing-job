package main

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	lockKeyPrefix = "billing:lock:"
	lockTimeout   = 30 * time.Second
	lockRetryTime = 5 * time.Second
)

type RedisLock struct {
	client     *redis.Client
	key        string
	value      string
	ctx        context.Context
	cancelFunc context.CancelFunc
}

// NewRedisLock 创建一个新的分布式锁
func NewRedisLock(client *redis.Client, key string) *RedisLock {
	ctx, cancel := context.WithCancel(context.Background())
	return &RedisLock{
		client:     client,
		key:        lockKeyPrefix + key,
		value:      fmt.Sprintf("%d", time.Now().UnixNano()), // 使用时间戳作为锁的值
		ctx:        ctx,
		cancelFunc: cancel,
	}
}

// TryLock 尝试获取锁
func (rl *RedisLock) TryLock() (bool, error) {
	// 使用 SET NX 命令尝试获取锁
	success, err := rl.client.SetNX(rl.ctx, rl.key, rl.value, lockTimeout).Result()
	if err != nil {
		return false, fmt.Errorf("failed to acquire lock: %v", err)
	}

	if success {
		// 启动自动续期
		go rl.autoRenew()
	}

	return success, nil
}

// Lock 获取锁，如果获取失败会重试
func (rl *RedisLock) Lock() error {
	for {
		success, err := rl.TryLock()
		if err != nil {
			return err
		}
		if success {
			return nil
		}
		time.Sleep(lockRetryTime)
	}
}

// Unlock 释放锁
func (rl *RedisLock) Unlock() error {
	// 使用 Lua 脚本确保原子性操作
	script := `
		if redis.call("get", KEYS[1]) == ARGV[1] then
			return redis.call("del", KEYS[1])
		else
			return 0
		end
	`

	// 停止自动续期
	rl.cancelFunc()

	// 执行 Lua 脚本
	result, err := rl.client.Eval(rl.ctx, script, []string{rl.key}, rl.value).Result()
	if err != nil {
		return fmt.Errorf("failed to release lock: %v", err)
	}

	if result.(int64) == 0 {
		return fmt.Errorf("lock not held by this client")
	}

	return nil
}

// autoRenew 自动续期
func (rl *RedisLock) autoRenew() {
	ticker := time.NewTicker(lockTimeout / 3)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			// 续期锁
			success, err := rl.client.SetXX(rl.ctx, rl.key, rl.value, lockTimeout).Result()
			if err != nil || !success {
				return
			}
		case <-rl.ctx.Done():
			return
		}
	}
}

// IsLocked 检查锁是否被持有
func (rl *RedisLock) IsLocked() (bool, error) {
	val, err := rl.client.Get(rl.ctx, rl.key).Result()
	if err == redis.Nil {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return val == rl.value, nil
}

// 使用示例
func ExampleUsage() {
	client := redis.NewClient(&redis.Options{
		Addr:     "localhost:6379",
		Password: "",
		DB:       0,
	})

	// 创建锁
	lock := NewRedisLock(client, "my-lock")

	// 尝试获取锁
	err := lock.Lock()
	if err != nil {
		fmt.Printf("Failed to acquire lock: %v\n", err)
		return
	}
	defer lock.Unlock()

	// 执行需要加锁的操作
	// ...

	// 检查锁状态
	isLocked, err := lock.IsLocked()
	if err != nil {
		fmt.Printf("Failed to check lock status: %v\n", err)
		return
	}
	fmt.Printf("Lock is held: %v\n", isLocked)
}
