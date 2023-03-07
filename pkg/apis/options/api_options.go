package options

import "time"

type API struct {
	Host       string
	Port       int
	PathPrefix string
	Timeout    time.Duration
}
