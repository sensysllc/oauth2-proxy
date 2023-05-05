package postgres

import (
	"context"

	"github.com/oauth2-proxy/oauth2-proxy/v7/pkg/apis/options"
	"github.com/oauth2-proxy/oauth2-proxy/v7/pkg/encryption"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

type PtgStore struct {
	configuration *options.Postgres
	db            *gorm.DB
	cipher        encryption.Cipher
}

type provider struct {
	ID           string `gorm:"embedded"`
	ProviderConf string // datatypes.JSON
}

func runMigrationsAndSetUpCipher(db *gorm.DB, schema string) (encryption.Cipher, error) {
	res := db.Exec("create schema if not exists  " + schema)
	if res.Error != nil {
		return nil, res.Error
	}

	err := db.AutoMigrate(&provider{})
	if err != nil {
		return nil, err
	}

	secret := make([]byte, 32)

	cstd, err := encryption.NewCFBCipher(secret)
	cb64 := encryption.NewBase64Cipher(cstd)
	if err != nil {
		return nil, err
	}
	return cb64, nil
}

func NewPostgresStore(c options.Postgres) (*PtgStore, error) {
	db, err := gorm.Open(postgres.Open(c.ConnectionString()), &gorm.Config{})
	if err != nil {
		return nil, err
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}
	sqlDB.SetMaxOpenConns(c.MaxConnections)

	cb64, err := runMigrationsAndSetUpCipher(db, c.Schema)
	if err != nil {
		return nil, err
	}

	ps := &PtgStore{
		configuration: &c,
		db:            db,
		cipher:        cb64,
	}

	return ps, nil
}

func (ps *PtgStore) encryptProviderConfig(providerConf []byte) ([]byte, error) {
	encryptedData, err := ps.cipher.Encrypt(providerConf)
	if err != nil {
		return nil, err
	}
	return encryptedData, nil
}

func (ps *PtgStore) decryptProviderConfig(providerConf []byte) ([]byte, error) {
	decrytedData, err := ps.cipher.Decrypt(providerConf)
	if err != nil {
		return nil, err
	}
	return decrytedData, nil
}

func (ps *PtgStore) Create(ctx context.Context, id string, providerconf []byte) error {

	encryptedProviderConf, err := ps.encryptProviderConfig(providerconf)
	if err != nil {
		return newError(err)
	}
	provider := provider{ID: id, ProviderConf: string(encryptedProviderConf)}
	res := ps.db.WithContext(ctx).Create(&provider)
	if res.Error != nil {
		return newError(res.Error)
	}

	return nil
}

// if not found affected rows=0
func (ps *PtgStore) Update(ctx context.Context, id string, providerconf []byte) error {

	encryptedProviderConf, err := ps.cipher.Encrypt(providerconf)
	if err != nil {
		return newError(err)
	}
	res := ps.db.WithContext(ctx).Model(&provider{}).Where("id = ?", id).Update("provider_conf", encryptedProviderConf)
	if res.Error != nil {
		return newError(res.Error)
	}
	if res.RowsAffected == 0 {
		return NewNotFoundError("provider conf entry does not exist")
	}
	return nil
}

func (ps *PtgStore) Get(ctx context.Context, id string) (string, error) {

	var prov = &provider{
		ID: id,
	}
	res := ps.db.WithContext(ctx).First(prov)
	if res.Error != nil {
		return "", newError(res.Error)
	}
	providerConf, err := ps.decryptProviderConfig([]byte(prov.ProviderConf))
	if err != nil {
		return "", newError(err)
	}
	return string(providerConf), nil
}

func (ps *PtgStore) Delete(ctx context.Context, id string) error {

	res := ps.db.WithContext(ctx).Where("id = ?", id).Delete(&provider{})
	if res.Error != nil {
		return newError(res.Error)
	}
	if res.RowsAffected == 0 {
		return NewNotFoundError("config entry not found")
	}
	return nil
}
