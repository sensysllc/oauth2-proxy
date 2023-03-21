package configloader

import (
	"fmt"
	"github.com/oauth2-proxy/oauth2-proxy/v7/pkg/logger"

	"github.com/oauth2-proxy/oauth2-proxy/v7/pkg/apis/options"
	"github.com/oauth2-proxy/oauth2-proxy/v7/providers"
)

type Loader struct {
	providersConf   options.Providers             // providers configuration that has been loaded from file at path loader.conf.ProvidersFile
	providers       map[string]providers.Provider // providers map, key is provider id
	defaultProvider string
}

func New(conf options.Providers, defaultProvider string) (*Loader, error) {
	loader := &Loader{
		providersConf:   conf,
		defaultProvider: defaultProvider,
	}
	loader.providers = make(map[string]providers.Provider)

	for _, providerConf := range loader.providersConf {
		provider, err := providers.NewProvider(providerConf)
		if providerConf.ID == "" {
			return nil, fmt.Errorf("provider ID is not provided")
		}
		if err != nil {
			return nil, fmt.Errorf("invalid provider config(id=%s): %s", providerConf.ID, err.Error())
		}
		loader.providers[providerConf.ID] = provider
	}
	// We do not need to check len(loader.providersConf) > 0 since
	// it has already been done when validating the config.
	if defaultProvider == "" {
		loader.defaultProvider = loader.providersConf[0].ID
	}

	// Ensure the defaultProvider exist
	_, err := loader.Load(loader.defaultProvider)
	if err != nil {
		return nil, fmt.Errorf("invalid default-provider: %v", err.Error())
	}

	return loader, nil
}

func (l *Loader) Load(id string) (providers.Provider, error) {
	if id == "" {
		id = l.defaultProvider
	}

	if tnt, ok := l.providers[id]; ok {
		return tnt, nil
	} else {
		return nil, fmt.Errorf("no provider found with id='%s'", id)
	}
}

func (l *Loader) GetByIssuerURL(url string) (providers.Provider, error) {
	for _, provider := range l.providers {
		issuerURL, err := provider.GetIssuerURL()
		if err != nil {
			logger.Errorf(
				"could not get the issuerURL of the provider %s: %s",
				provider.Data().ProviderName, err.Error(),
			)
			continue
		}
		if issuerURL == url {
			return provider, nil
		}
	}

	return nil, fmt.Errorf("could not get a provider matching the issuerURL %s", url)
}
