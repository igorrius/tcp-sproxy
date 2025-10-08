package integration_tests

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/stretchr/testify/require"
)

func TestRedisProxy(t *testing.T) {
	// Skip this test if not running in Docker Compose environment
	_, err := net.LookupHost("proxy-client")
	if err != nil {
		t.Skip("Skipping integration test - proxy-client not available (not running in Docker Compose environment)")
	}

	ctx := context.Background()

	rdb := redis.NewClient(&redis.Options{
		Addr:        "proxy-client:8082",
		ReadTimeout: 30 * time.Second,
	})

	// Wait for the proxy to be ready
	time.Sleep(5 * time.Second)

	_, err = rdb.Ping(ctx).Result()
	require.NoError(t, err)

	err = rdb.Set(ctx, "key", "value", 0).Err()
	require.NoError(t, err)

	val, err := rdb.Get(ctx, "key").Result()
	require.NoError(t, err)
	require.Equal(t, "value", val)
}
