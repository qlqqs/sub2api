package service

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestRefreshCachedOpenAIModelsRechecksFreshCache(t *testing.T) {
	s := &OpenAIGatewayService{}
	const key = "models-recheck"
	old := &OpenAIModelsResponse{Body: []byte(`{"models":[]}`), upstreamETag: "old"}
	s.openAIModelsCache.set(key, old, time.Now().Add(-openAIModelsCacheTTL-time.Minute))
	_, state := s.openAIModelsCache.get(key, time.Now())
	require.Equal(t, openAIModelsCacheStale, state)

	// Another refresh finishes after the caller reads stale data, before this
	// caller enters singleflight. The completed result must be reused.
	fresh := &OpenAIModelsResponse{Body: []byte(`{"models":[{"slug":"new"}]}`), upstreamETag: "new"}
	s.openAIModelsCache.set(key, fresh, time.Now())
	var calls atomic.Int32
	result := <-s.refreshCachedOpenAIModels(key, openAIModelsRequest{}, func(context.Context, string) (*OpenAIModelsResponse, error) {
		calls.Add(1)
		return old, nil
	})
	require.NoError(t, result.Err)
	require.Same(t, fresh, result.Val)
	require.Zero(t, calls.Load())
}

func TestRefreshCachedOpenAIModelsStillRevalidatesStaleETag(t *testing.T) {
	s := &OpenAIGatewayService{}
	const key = "models-stale"
	cached := &OpenAIModelsResponse{Body: []byte(`{"models":[]}`), upstreamETag: "etag-old"}
	s.openAIModelsCache.set(key, cached, time.Now().Add(-openAIModelsCacheTTL-time.Minute))
	var calls atomic.Int32
	var sentETag string
	result := <-s.refreshCachedOpenAIModels(key, openAIModelsRequest{}, func(_ context.Context, etag string) (*OpenAIModelsResponse, error) {
		calls.Add(1)
		sentETag = etag
		return &OpenAIModelsResponse{NotModified: true}, nil
	})
	require.NoError(t, result.Err)
	require.Equal(t, int32(1), calls.Load())
	require.Equal(t, "etag-old", sentETag)
	require.Same(t, cached, result.Val)
	_, state := s.openAIModelsCache.get(key, time.Now())
	require.Equal(t, openAIModelsCacheFresh, state)
}
