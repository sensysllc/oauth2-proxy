package options

import (
	"fmt"
	"time"
)

type Postgres struct {
	Host           string
	Port           uint16
	Database       string
	Schema         string
	User           string
	Password       string
	Timeout        time.Duration
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
