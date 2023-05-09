package middleware

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/bluele/gcache"
	"github.com/oauth2-proxy/oauth2-proxy/v7/pkg/apis/options"
	"github.com/oauth2-proxy/oauth2-proxy/v7/pkg/providerloader"
	"github.com/oauth2-proxy/oauth2-proxy/v7/providers"
)

type ProviderLoaderDecorator struct {
	cache gcache.Cache
	pl    providerloader.Loader
}

func NewProviderLoaderDecorator(pl providerloader.Loader, cf options.ProviderCache) providerloader.Loader {
	gc := gcache.New(cf.CacheLimit).
		LRU().
		Expiration(time.Duration(cf.CacheDuration) * time.Second).
		Build()

	return ProviderLoaderDecorator{
		cache: gc,
		pl:    pl,
	}

}

func (pld ProviderLoaderDecorator) Load(ctx context.Context, id string) (providers.Provider, error) {
	var provider providers.Provider
	var ok bool
	value, err := pld.cache.Get(id)
	if err != nil {
		if errors.Is(err, gcache.KeyNotFoundError) {
			provider, err = pld.pl.Load(ctx, id)
			if err != nil {
				return nil, fmt.Errorf("provider loader failed: %w", err)
			}
			pld.cache.Set(id, provider)
		} else {
			return nil, fmt.Errorf("can't get provider from in-memory cache:%w", err)
		}
	} else {
		provider, ok = value.(providers.Provider)
		if !ok {
			return nil, fmt.Errorf("can't get find provider in cache")
		}
	}

	return provider, nil
}
