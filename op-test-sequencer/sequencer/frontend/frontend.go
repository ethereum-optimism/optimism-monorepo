package frontend

import "context"

type Backend interface {
	Hello(ctx context.Context, name string) (string, error)
}

type AdminFrontend struct {
	Backend Backend
}

func (af *AdminFrontend) Hello(ctx context.Context, name string) (string, error) {
	return af.Backend.Hello(ctx, name)
}
