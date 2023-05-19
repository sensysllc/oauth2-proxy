package redirect

type noopValidator struct{}

func (noop *noopValidator) IsValidRedirect(redirect string) bool {
	return true
}

func NewNoopValidator() Validator {
	return &noopValidator{}
}
