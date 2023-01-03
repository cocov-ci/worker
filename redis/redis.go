package redis

import (
	"context"
	"encoding/json"
	"github.com/go-redis/redis/v8"
	"go.uber.org/zap"
	"sync"
	"time"
)

type Client interface {
	Next() *Job
	Ping() error
	Start()
	Stop()
	Wait()
}

func New(url string) (Client, error) {
	opts, err := redis.ParseURL(url)
	if err != nil {
		return nil, err
	}

	redisClient := redis.NewClient(opts)
	c := &client{
		c:    redisClient,
		mu:   &sync.Mutex{},
		stop: make(chan bool),
		want: make(chan int8, 100),
		got:  make(chan *Job),
		log:  zap.L().With(zap.String("facility", "redis")),
	}
	if err = c.Ping(); err != nil {
		return nil, err
	}

	c.log.Info("Connected to Redis", zap.String("host", opts.Addr))
	return c, nil
}

type client struct {
	c *redis.Client

	mu      *sync.Mutex
	stopped bool
	stop    chan bool
	want    chan int8
	got     chan *Job
	log     *zap.Logger
}

func (c *client) Ping() error {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	return c.c.Ping(ctx).Err()
}

func (c *client) Stop() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.stopped {
		return
	}

	c.log.Info("Stopping...")
	c.stopped = true
	close(c.want)
	c.log.Info("Stopped. Stand by for drainage...")
}

func (c *client) Wait() {
	<-c.stop
}

func (c *client) Start() {
	if c.stopped {
		return
	}

	for {
		_, ok := <-c.want
		if !ok {
			c.log.Info("Consumed all requests")
			break
		}

		c.mu.Lock()
		if c.stopped {
			c.mu.Unlock()
			continue
		}

		c.mu.Unlock()
		backoff := 1
		for {
			dataSlice, err := c.c.BLPop(context.Background(), 5*time.Second, "cocov:checks").Result()
			if err == redis.Nil {
				backoff = 1
				continue
			} else if err != nil {
				toSleep := time.Duration(backoff) * time.Second
				c.log.Error("BLPop failed", zap.Error(err), zap.Duration("backoff", toSleep))
				time.Sleep(toSleep)
				backoff *= 2
				continue
			}
			backoff = 1

			var job *Job
			if err = json.Unmarshal([]byte(dataSlice[1]), &job); err != nil {
				c.log.Error("Failed parsing job descriptor", zap.Error(err), zap.String("contents", dataSlice[1]))
				c.pushDead(dataSlice[1])
				continue
			}

			c.got <- job
			break
		}
	}

	c.log.Info("Finished run loop")
	c.mu.Lock()
	c.stopped = true
	close(c.got)
	close(c.stop)
	c.mu.Unlock()
}

func (c *client) pushDead(data string) {
	if err := c.c.RPush(context.Background(), "cocov:checks:dead", data).Err(); err != nil {
		c.log.Error("Failed pushing dead item", zap.Error(err), zap.String("data", data))
	}
}

func (c *client) Next() *Job {
	c.mu.Lock()
	if c.stopped {
		c.mu.Unlock()
		return nil
	}
	c.want <- 0
	c.mu.Unlock()
	return <-c.got
}
