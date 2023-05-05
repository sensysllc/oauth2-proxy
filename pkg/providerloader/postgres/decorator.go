package postgres

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/oauth2-proxy/oauth2-proxy/v7/pkg/apis/options"
	"github.com/oauth2-proxy/oauth2-proxy/v7/pkg/encryption"
)

// this is a decorator/wrapper over ConfigStore
// it encrypts the keycloak secret before storing in db and cache.
type encryptionDecorator struct {
	ConfigStore
	cipher encryption.Cipher
}

func EncryptionDecorator(c ConfigStore, secret string) (ConfigStore, error) {

	cstd, err := encryption.NewCFBCipher([]byte(secret))
	if err != nil {
		return nil, fmt.Errorf("unable to create cipher from secret: %w", err)
	}
	cb64 := encryption.NewBase64Cipher(cstd)
	return &encryptionDecorator{
		ConfigStore: c,
		cipher:      cb64,
	}, nil
}

type encryptOrDecryptFunc func([]byte) ([]byte, error)

func (en *encryptionDecorator) encryptOrDecryptClientSecret(providerconf []byte, action encryptOrDecryptFunc) ([]byte, error) {
	var providerConf *options.Provider

	err := json.Unmarshal(providerconf, &providerConf)
	if err != nil {
		return nil, fmt.Errorf("error while decoding JSON into provider config. %w", err)
	}

	UpdatedSecret, err := action([]byte(providerConf.ClientSecret))
	if err != nil {
		return nil, err
	}
	providerConf.ClientSecret = string(UpdatedSecret)
	UpdateProviderconf, err := json.Marshal(providerConf)
	if err != nil {
		return nil, err
	}

	return UpdateProviderconf, nil
}

func (en *encryptionDecorator) Create(ctx context.Context, id string, providerconf []byte) error {
	updatedProviderconf, err := en.encryptOrDecryptClientSecret(providerconf, en.cipher.Encrypt)
	if err != nil {
		return fmt.Errorf("encryption error: %w", err)
	}
	return en.ConfigStore.Create(ctx, id, updatedProviderconf)
}

func (en *encryptionDecorator) Update(ctx context.Context, id string, providerconf []byte) error {
	updatedProviderconf, err := en.encryptOrDecryptClientSecret(providerconf, en.cipher.Encrypt) // secret in updates is encrypted
	if err != nil {
		return fmt.Errorf("encryption error: %w", err) // return error in case of unsuccessful
	}
	return en.ConfigStore.Update(ctx, id, updatedProviderconf)
}

func (en *encryptionDecorator) Get(ctx context.Context, id string) (string, error) {
	providerconf, err := en.ConfigStore.Get(ctx, id)
	if err != nil {
		return "", err
	}

	UpdatedProviderconf, err := en.encryptOrDecryptClientSecret([]byte(providerconf), en.cipher.Decrypt)
	if err != nil {
		return "", err
	}

	return string(UpdatedProviderconf), nil
}
