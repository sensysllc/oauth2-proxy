package options

import "time"

type Redis struct {
	Addr     string
	Expiry   time.Duration
	Password string
	DB       int
	Prefix   string
}
