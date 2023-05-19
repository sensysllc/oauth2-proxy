package middleware

import (
	"context"
	"reflect"
	"testing"

	"github.com/oauth2-proxy/oauth2-proxy/v7/pkg/apis/options"
	"github.com/oauth2-proxy/oauth2-proxy/v7/pkg/providerloader"
	"github.com/oauth2-proxy/oauth2-proxy/v7/pkg/providerloader/configloader"
	"github.com/oauth2-proxy/oauth2-proxy/v7/providers"
)

func Test_Load(t *testing.T) {
	l, _ := configloader.New(options.Providers{
		{
			ID:   "dummy",
			Type: "keycloak",
		},
	})

	wantProvider, _ := providers.NewProvider(options.Provider{
		ID:   "dummy",
		Type: "keycloak",
	})

	pld := NewProviderLoaderDecorator(l, options.ProviderCache{
		CacheDuration: 20,
		CacheLimit:    10,
	})

	tests := []struct {
		name     string
		tenantid string
		loader   providerloader.Loader
		provider providers.Provider
		wantErr  bool
	}{
		{
			"no error",
			"dummy",
			pld,
			wantProvider,
			false,
		},
		{
			"with error from loader",
			"xyz",
			pld,
			nil,
			true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			gotProvider, err := pld.Load(ctx, tt.tenantid)
			if err == nil && !tt.wantErr && !reflect.DeepEqual(gotProvider, wantProvider) {
				t.Errorf("provider loader decorator  = %v, want %v", gotProvider, wantProvider)
			} else if err != nil && !tt.wantErr {
				t.Errorf("provider loader decorator, got error: '%v'", err)
			}
		})
	}
}
