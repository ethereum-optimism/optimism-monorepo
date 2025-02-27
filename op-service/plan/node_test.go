package plan_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ethereum-optimism/optimism/op-service/plan"
)

func TestNode(t *testing.T) {
	t.Run("no func", func(t *testing.T) {
		x := new(plan.Lazy[uint64])
		y := new(plan.Lazy[uint64])
		y.DependOn(x)
		_, err := y.Eval(context.Background())
		require.ErrorIs(t, err, plan.ErrNotReady, "y does not define how it is computed")
	})
	t.Run("missing value", func(t *testing.T) {
		x := new(plan.Lazy[uint64])

		require.PanicsWithError(t, plan.ErrNotReady.Error(), func() {
			x.Value()
		})

		y := new(plan.Lazy[uint64])
		y.DependOn(x)
		y.Fn(func(ctx context.Context) (uint64, error) {
			return x.Value(), nil
		})
		_, err := y.Eval(context.Background())
		require.ErrorIs(t, err, plan.ErrNotReady, "x is undefined still")
	})
	t.Run("no error or value yet", func(t *testing.T) {
		x := new(plan.Lazy[uint64])
		err := x.Err()
		require.ErrorIs(t, err, plan.ErrNotReady)

		require.PanicsWithError(t, plan.ErrNotReady.Error(), func() {
			x.Value()
		})
		_, err = x.Get()
		require.ErrorIs(t, err, plan.ErrNotReady)
	})
	t.Run("simple", func(t *testing.T) {
		x := new(plan.Lazy[uint64])
		y := new(plan.Lazy[uint64])
		z := new(plan.Lazy[uint64])
		y.DependOn(x)
		y.Fn(func(ctx context.Context) (uint64, error) {
			return x.Value() + 10, nil
		})
		z.DependOn(y)
		z.Fn(func(ctx context.Context) (uint64, error) {
			return y.Value() * 2, nil
		})
		_, err := z.Eval(context.Background())
		require.ErrorIs(t, err, plan.ErrNotReady, "x is undefined still")

		x.Set(3)
		out, err := z.Eval(context.Background())
		require.NoError(t, err)
		require.Equal(t, uint64((3+10)*2), out)

		x.Set(30)
		out, err = z.Eval(context.Background())
		require.NoError(t, err)
		require.Equal(t, uint64((30+10)*2), out, "x should have affected z")
	})
	t.Run("diamond", func(t *testing.T) {
		x := new(plan.Lazy[uint64])
		yA := new(plan.Lazy[uint64])
		yB := new(plan.Lazy[uint32]) // nodes can have different types
		z := new(plan.Lazy[uint64])
		yA.DependOn(x)
		yB.DependOn(x)
		yA.Fn(func(ctx context.Context) (uint64, error) {
			return x.Value() + 10, nil
		})
		yB.Fn(func(ctx context.Context) (uint32, error) {
			return uint32(x.Value()) * 20, nil
		})
		z.DependOn(yA, yB)
		z.Fn(func(ctx context.Context) (uint64, error) {
			return (yA.Value() + 2) * uint64(yB.Value()), nil
		})
		x.Set(30)
		out, err := z.Eval(context.Background())
		require.NoError(t, err)
		require.Equal(t, uint64((30+10)+2)*(30*20), out, "x should have affected z")

		x.Set(100)
		out, err = z.Eval(context.Background())
		require.NoError(t, err)
		require.Equal(t, uint64((100+10)+2)*(100*20), out, "x should have affected z again")

		yA.Set(1000)
		out, err = z.Eval(context.Background())
		require.NoError(t, err)
		require.Equal(t, uint64(1000+2)*(100*20), out, "yA override should work")

		s := z.String()
		require.Equal(t, `*plan.Lazy[uint64](*plan.Lazy[uint64], *plan.Lazy[uint32])`, s)
	})

	t.Run("close", func(t *testing.T) {
		x := new(plan.Lazy[uint64])
		y := new(plan.Lazy[int32])
		z := new(plan.Lazy[int64])
		q := new(plan.Lazy[uint64])
		y.DependOn(x)
		y.Fn(func(ctx context.Context) (int32, error) {
			return int32(x.Value()) + 10, nil
		})
		z.DependOn(y)
		z.Fn(func(ctx context.Context) (int64, error) {
			return int64(y.Value()) * 2, nil
		})
		q.DependOn(z)
		q.Fn(func(ctx context.Context) (uint64, error) {
			return uint64(z.Value()) + 5, nil
		})
		x.Set(3)
		out, err := q.Eval(context.Background())
		require.NoError(t, err)
		require.Equal(t, uint64(((3+10)*2)+5), out)

		// Closing y should not affect x,
		// but should invalidate q
		y.Close()

		out, err = x.Eval(context.Background())
		require.NoError(t, err)
		require.Equal(t, uint64(3), out)

		_, err = q.Eval(context.Background())
		require.ErrorIs(t, err, plan.ErrNotReady)
	})
	t.Run("simple error", func(t *testing.T) {
		x := new(plan.Lazy[uint64])
		testErr1 := errors.New("foobar 1")
		x.SetError(testErr1)
		require.ErrorIs(t, x.Err(), testErr1)
		testErr2 := errors.New("foobar 2")
		x.SetError(testErr2)
		require.ErrorIs(t, x.Err(), testErr2)
	})
	t.Run("upstream error", func(t *testing.T) {
		x := new(plan.Lazy[uint64])
		y := new(plan.Lazy[uint64])
		z := new(plan.Lazy[uint64])
		y.DependOn(x)
		testErr := errors.New("foobar")
		y.Fn(func(ctx context.Context) (uint64, error) {
			return 42, testErr
		})
		z.DependOn(y)
		z.Fn(func(ctx context.Context) (uint64, error) {
			return y.Value() * 2, nil
		})
		x.Set(3)
		_, err := z.Eval(context.Background())
		require.ErrorIs(t, err, testErr)
		require.ErrorContains(t, err, "upstream dep")

		require.ErrorIs(t, y.Err(), testErr)
		require.Equal(t, uint64(42), y.Value())
		val, err := y.Get()
		require.ErrorIs(t, err, testErr)
		require.Equal(t, uint64(42), val, "value is registered, even if along with error")
		require.ErrorIs(t, z.Err(), testErr)
		require.ErrorContains(t, z.Err(), "upstream")
		require.Equal(t, uint64(0), z.Value(), "value does not propagate")

		testErr2 := errors.New("foobar 2")
		x.SetError(testErr2) // invalidates the error returned by y
		_, err = z.Eval(context.Background())
		require.ErrorIs(t, err, testErr2)
	})
}
