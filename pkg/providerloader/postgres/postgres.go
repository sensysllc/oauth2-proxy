package postgres

import (
	"context"
	"fmt"

	"github.com/oauth2-proxy/oauth2-proxy/v7/pkg/apis/options"
	"gorm.io/datatypes"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

type PtgStore struct {
	configuration *options.Postgres
	db            *gorm.DB
}

type provider struct {
	ID           string `gorm:"type:VARCHAR(255);primarykey"`
	ProviderConf datatypes.JSON
	// ProviderConf options.Provider `json:"providerConf"`
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

	res := db.Exec("create schema if not exists  " + c.Schema)
	if res.Error != nil {
		return nil, res.Error
	}

	err = db.AutoMigrate(&provider{})
	if err != nil {
		return nil, err
	}

	ps := &PtgStore{
		configuration: &c,
		db:            db,
	}

	return ps, nil
}

func (ps *PtgStore) Create(ctx context.Context, id string, providerconf []byte) error {
	ctx, cancel := context.WithTimeout(ctx, ps.configuration.Timeout)
	defer cancel()

	provider := provider{ID: id, ProviderConf: providerconf}
	res := ps.db.WithContext(ctx).Create(&provider)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return fmt.Errorf("provider config entry not created")
	}

	return nil
}

func (ps *PtgStore) Update(ctx context.Context, id string, providerconf []byte) error {
	ctx, cancel := context.WithTimeout(ctx, ps.configuration.Timeout)
	defer cancel()

	res := ps.db.WithContext(ctx).Model(&provider{}).Where("id = ?", id).Update("provider_conf", providerconf)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return fmt.Errorf("provider conf entry does not exist")
	}
	return nil
}

func (ps *PtgStore) Get(ctx context.Context, id string) (string, error) {

	ctx, cancel := context.WithTimeout(ctx, ps.configuration.Timeout)
	defer cancel()

	var prov = &provider{}
	prov.ID = id
	res := ps.db.WithContext(ctx).First(prov)
	if res.Error != nil {
		return "", res.Error
	}
	return string(prov.ProviderConf), nil
}

func (ps *PtgStore) Delete(ctx context.Context, id string) error {
	ctx, cancel := context.WithTimeout(ctx, ps.configuration.Timeout)
	defer cancel()

	res := ps.db.WithContext(ctx).Where("id = ?", id).Delete(&provider{})
	if res.Error != nil {
		return res.Error
	}
	return nil
}
