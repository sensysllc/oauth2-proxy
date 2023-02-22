package single

import (
	"reflect"
	"testing"

	"github.com/oauth2-proxy/oauth2-proxy/v7/pkg/apis/options"
	"github.com/oauth2-proxy/oauth2-proxy/v7/providers"
)

func TestNew(t *testing.T) {

	prov, _ := providers.NewProvider(options.Provider{

		ID:   "dummy",
		Type: "keycloak",
	})
	tests := []struct {
		name    string
		conf    options.Provider
		want    *Loader
		wantErr bool
	}{
		{
			"config loader",
			options.Provider{

				ID:   "dummy",
				Type: "xxxx",
			},
			nil,
			true,
		},
		{
			"config loader",
			options.Provider{

				ID:   "dummy",
				Type: "keycloak",
			},
			&Loader{
				config: &options.Provider{

					ID:   "dummy",
					Type: "keycloak",
				},
				provider: prov,
			},
			false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := New(tt.conf)

			if err == nil && !tt.wantErr && !reflect.DeepEqual(got, tt.want) {
				t.Errorf("New config loader  = %v, want %v", got, tt.want)
			} else if err != nil && !tt.wantErr {
				t.Errorf("New config loader, got error: '%v'", err)
			}
		})
	}

}

func TestLoad(t *testing.T) {
	l := &Loader{
		config: &options.Provider{

			ID:   "dummy",
			Type: "keycloak",
		},
		provider: &providers.KeycloakProvider{},
	}
	tests := []struct {
		name    string
		id      string
		want    providers.Provider
		wantErr bool
	}{
		{
			"load",
			"dummy",
			&providers.KeycloakProvider{},
			false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, _ := l.Load("")

			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf(" load returned  = %v, want %v", got, tt.want)
			}
		})
	}
}
