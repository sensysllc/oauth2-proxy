package postgres

import "context"

// ConfigStore is an interface defined that allows any kind of database or store
// for saving providers configuration.
// Defining a generic interface allows for smooth testing as well.
type ConfigStore interface {
	Create(ctx context.Context, id string, providerConfig []byte) error
	Update(ctx context.Context, id string, providerConfig []byte) error
	Get(ctx context.Context, id string) (string, error)
	Delete(ctx context.Context, id string) error
}

// This is an implementation of ConfigStore interface which is used in testing scenarios.
// As the name implies it is a fake store and looks for implementation of funcs defined in original
// interface.
type fakeConfigStore struct {
	CreateFunc func(ctx context.Context, id string, providerConfig []byte) error
	UpdateFunc func(ctx context.Context, id string, providerConfig []byte) error
	GetFunc    func(ctx context.Context, id string) (string, error)
	DeleteFunc func(ctx context.Context, id string) error
}

func (f fakeConfigStore) Create(ctx context.Context, id string, providerConfig []byte) error {
	if f.CreateFunc != nil {
		return f.CreateFunc(ctx, id, providerConfig)
	}
	return nil
}

func (f fakeConfigStore) Update(ctx context.Context, id string, providerConfig []byte) error {
	if f.UpdateFunc != nil {
		return f.UpdateFunc(ctx, id, providerConfig)
	}
	return nil
}

func (f fakeConfigStore) Get(ctx context.Context, id string) (string, error) {
	if f.GetFunc != nil {
		return f.GetFunc(ctx, id)
	}
	return "", nil
}

func (f fakeConfigStore) Delete(ctx context.Context, id string) error {
	if f.DeleteFunc != nil {
		return f.DeleteFunc(ctx, id)
	}
	return nil
}
