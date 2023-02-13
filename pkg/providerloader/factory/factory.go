package factory

import (
	"fmt"

	"github.com/oauth2-proxy/oauth2-proxy/v7/pkg/apis/options"
	"github.com/oauth2-proxy/oauth2-proxy/v7/pkg/providerloader"
	"github.com/oauth2-proxy/oauth2-proxy/v7/pkg/providerloader/configloader"
	"github.com/oauth2-proxy/oauth2-proxy/v7/pkg/providerloader/single"
)

// factory function for types.Loader interface
func NewLoader(opts *options.Options) (providerloader.Loader, error) {
	conf := opts.ProviderLoader
	switch conf.Type {
	case "config":
		return configloader.New(opts.Providers)
	case "", "single": // empty value in case we're using legacy opts
		return single.New(opts.Providers[0])
	default:
		return nil, fmt.Errorf("invalid tenant loader type '%s'", conf.Type)
	}
}
