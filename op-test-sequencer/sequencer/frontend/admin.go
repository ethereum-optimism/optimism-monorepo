package frontend

import "context"

type AdminBackend interface {
	Hello(ctx context.Context, name string) (string, error)
}

type AdminFrontend struct {
	Backend AdminBackend
}

func (af *AdminFrontend) Hello(ctx context.Context, name string) (string, error) {
	return af.Backend.Hello(ctx, name)
}
