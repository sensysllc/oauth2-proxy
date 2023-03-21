package single

import (
	"fmt"

	"github.com/oauth2-proxy/oauth2-proxy/v7/pkg/apis/options"
	"github.com/oauth2-proxy/oauth2-proxy/v7/providers"
)

type Loader struct {
	Config   *options.Provider
	Provider providers.Provider
}

func New(conf options.Provider) (*Loader, error) {
	provider, err := providers.NewProvider(conf)
	if err != nil {
		return nil, fmt.Errorf("unable to create new provider: %w", err)
	}
	return &Loader{
		Config:   &conf,
		Provider: provider,
	}, nil
}

func (l *Loader) Load(_ string) (providers.Provider, error) {
	return l.Provider, nil
}

func (l *Loader) GetByIssuerURL(url string) (providers.Provider, error) {
	issuerURL, err := l.Provider.GetIssuerURL()
	if err != nil {
		return nil, err
	}
	if issuerURL != url {
		return nil, fmt.Errorf("the issuerURL does not correspond match")
	}
	return l.Provider, nil
}
