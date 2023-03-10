package options

import (
	"fmt"
	"time"
)

type PostgresLoader struct {
	Postgres Postgres
	Redis    Redis
	API      API
}

type API struct {
	Host       string
	Port       int
	PathPrefix string
	Timeout    time.Duration
}

type Redis struct {
	Addr     string
	Expiry   time.Duration
	Password string
	Prefix   string
}

type Postgres struct {
	Host           string
	Port           uint16
	Database       string
	Schema         string
	User           string
	Password       string
	MaxConnections int
	SslMode        string
	SslRootCert    string
}

func (c *Postgres) ConnectionString() string {
	if c.SslRootCert == "" {
		return fmt.Sprintf("port=%v host=%s user=%s password=%s dbname=%s search_path=%s sslmode=%s", c.Port, c.Host, c.User, c.Password, c.Database, c.Schema, c.SslMode)
	}
	return fmt.Sprintf("port=%v host=%s user=%s password=%s dbname=%s search_path=%s sslmode=%s sslrootcert=%s", c.Port, c.Host, c.User, c.Password, c.Database, c.Schema, c.SslMode, c.SslRootCert)
}
