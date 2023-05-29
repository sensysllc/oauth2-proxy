package options

type ProviderCache struct {
	// It specifies the maximum number of items that the cache can hold.
	CacheLimit int

	// It defines the number of seconds after which the providers in in-memory cache will expire
	CacheDuration int
}
