package providerloader

import (
	"github.com/oauth2-proxy/oauth2-proxy/v7/providers"
)

type Loader interface {
	// id is provider id, which should be same as tenantId
	Load(id string) (providers.Provider, error)
}
