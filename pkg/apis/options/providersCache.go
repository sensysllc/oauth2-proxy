package options

type ProviderCache struct {
	// It defines capacity of keeping providers in an in-memory cache
	CacheLimit int

	// The time.duration in seconds after which the providers in in-memory cache will expire
	CacheDuration int
}
