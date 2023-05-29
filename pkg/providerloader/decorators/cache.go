package decorators

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/bluele/gcache"
	"github.com/oauth2-proxy/oauth2-proxy/v7/pkg/apis/options"
	"github.com/oauth2-proxy/oauth2-proxy/v7/pkg/providerloader"
	"github.com/oauth2-proxy/oauth2-proxy/v7/providers"
)

type inMemoryCache struct {
	cache gcache.Cache
	pl    providerloader.Loader
}

func WithCache(pl providerloader.Loader, cf options.ProviderCache) providerloader.Loader {
	gc := gcache.New(cf.CacheLimit).
		LRU().
		Expiration(time.Duration(cf.CacheDuration) * time.Second).
		Build()

	return inMemoryCache{
		cache: gc,
		pl:    pl,
	}

}
func (c inMemoryCache) Load(ctx context.Context, id string) (providers.Provider, error) {
	var provider providers.Provider
	var ok = true
	value, err := c.cache.Get(id)

	if err == nil {
		if provider, ok = value.(providers.Provider); ok {
			return provider, nil
		}
	}

	if err != nil && !errors.Is(err, gcache.KeyNotFoundError) {
		log.Printf("cache returned error: %v", err)
	} else if !ok {
		log.Printf("cache returned invalid provider interface type: %T", provider)
	}

	provider, err = c.pl.Load(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to load provider: %w", err)
	}

	err = c.cache.Set(id, provider)
	if err != nil {
		log.Printf("unable to store provider in cache: %v", err)
	}

	return provider, nil
}
