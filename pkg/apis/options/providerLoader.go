package options

type ProviderLoader struct {
	Type            string // possible values are "single" and "config" for now
	DefaultProvider string // default provider, default value "" will use the first one defined.
}
