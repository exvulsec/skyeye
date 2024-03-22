package datastore

import (
	"sync"

	"github.com/redis/go-redis/v9"

	"github.com/exvulsec/skyeye/config"
)

var redisInstance *RedisInstance

type RedisInstance struct {
	initializer func() any
	instance    any
	once        sync.Once
}

// Instance gets the singleton instance
func (i *RedisInstance) Instance() any {
	i.once.Do(func() {
		i.instance = i.initializer()
	})
	return i.instance
}

func initRedisClient() any {
	return redis.NewClient(&redis.Options{
		Addr:         config.Conf.Redis.Addr,
		Password:     config.Conf.Redis.Password,
		DB:           config.Conf.Redis.Database,
		MaxIdleConns: config.Conf.Redis.MaxIdleConns,
	})
}

func Redis() *redis.Client {
	return redisInstance.Instance().(*redis.Client)
}

func init() {
	redisInstance = &RedisInstance{initializer: initRedisClient}
}
