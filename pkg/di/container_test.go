package di_test

import (
	"errors"
	"rgb-game/pkg/di"
	"strings"
	"testing"
)

// ─── Helpers ─────────────────────────────────────────────────────────────────

type A struct{ Val string }
type B struct{ A *A }
type C struct{ B *B }

func newA() *A                    { return &A{Val: "a"} }
func newB(a *A) *B                { return &B{A: a} }
func newC(b *B) *C                { return &C{B: b} }
func newAWithErr() (*A, error)    { return &A{Val: "from-err-shape"}, nil }
func newAFailing() (*A, error)    { return nil, errors.New("constructor error") }

type MyIface interface{ Hello() string }
type MyImpl struct{}

func (m *MyImpl) Hello() string { return "hello" }
func newMyImpl() *MyImpl        { return &MyImpl{} }

// ─── Tests ───────────────────────────────────────────────────────────────────

func TestMultiDepChain(t *testing.T) {
	c := di.New()
	c.Provide(newA)
	c.Provide(newB)
	c.Provide(newC)

	var out *C
	if err := c.Resolve(&out); err != nil {
		t.Fatalf("Resolve failed: %v", err)
	}
	if out.B.A.Val != "a" {
		t.Errorf("unexpected val: %s", out.B.A.Val)
	}
}

func TestSingletonCached(t *testing.T) {
	calls := 0
	c := di.New()
	c.Provide(func() *A {
		calls++
		return &A{}
	})

	var a1, a2 *A
	c.Resolve(&a1)
	c.Resolve(&a2)

	if calls != 1 {
		t.Errorf("expected constructor called once, got %d", calls)
	}
	if a1 != a2 {
		t.Error("expected same singleton pointer")
	}
}

func TestErrorShape(t *testing.T) {
	c := di.New()
	c.Provide(newAWithErr)

	var a *A
	if err := c.Resolve(&a); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if a.Val != "from-err-shape" {
		t.Errorf("unexpected val: %s", a.Val)
	}
}

func TestConstructorError(t *testing.T) {
	c := di.New()
	c.Provide(newAFailing)

	var a *A
	err := c.Resolve(&a)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "constructor error") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestCleanupShapeAndLIFO(t *testing.T) {
	order := []string{}

	c := di.New()
	c.Provide(func() (*A, func(), error) {
		return &A{Val: "a"}, func() { order = append(order, "close-A") }, nil
	})
	c.Provide(func(a *A) (*B, func(), error) {
		return &B{A: a}, func() { order = append(order, "close-B") }, nil
	})

	var b *B
	if err := c.Resolve(&b); err != nil {
		t.Fatalf("Resolve: %v", err)
	}

	c.Close()

	// Cleanups must be LIFO: B registered after A, so B closes first.
	if len(order) != 2 || order[0] != "close-B" || order[1] != "close-A" {
		t.Errorf("unexpected cleanup order: %v", order)
	}
}

func TestProvideAs(t *testing.T) {
	c := di.New()
	c.ProvideAs(newMyImpl, (*MyIface)(nil))

	var iface MyIface
	if err := c.Resolve(&iface); err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if iface.Hello() != "hello" {
		t.Errorf("unexpected: %s", iface.Hello())
	}
}

func TestNamedBindings(t *testing.T) {
	c := di.New()
	c.Provide(func() *A { return &A{Val: "default"} })
	c.ProvideNamed("alt", func() *A { return &A{Val: "alt"} })

	var def, alt *A
	c.Resolve(&def)
	c.ResolveNamed("alt", &alt)

	if def.Val != "default" {
		t.Errorf("default: %s", def.Val)
	}
	if alt.Val != "alt" {
		t.Errorf("alt: %s", alt.Val)
	}
}

func TestTransientLifetime(t *testing.T) {
	calls := 0
	c := di.New()
	c.Provide(func() *A {
		calls++
		return &A{}
	}, di.WithLifetime(di.Transient))

	var a1, a2 *A
	c.Resolve(&a1)
	c.Resolve(&a2)

	if calls != 2 {
		t.Errorf("expected 2 constructor calls, got %d", calls)
	}
	if a1 == a2 {
		t.Error("expected different transient pointers")
	}
}

func TestScopedLifetime(t *testing.T) {
	c := di.New()
	c.Provide(func() *A { return &A{Val: "root-singleton"} })
	c.Provide(func(a *A) *B { return &B{A: a} }, di.WithLifetime(di.Scoped))

	scope1 := c.Scope()
	scope2 := c.Scope()

	var b1, b2 *B
	scope1.Resolve(&b1)
	scope1.Resolve(&b1) // second resolve within scope1 should return same instance

	scope2.Resolve(&b2)

	if b1 == b2 {
		t.Error("scoped: expected different instances across scopes")
	}
}

func TestScopedChildCloseDoesNotAffectParent(t *testing.T) {
	parentClosed := false
	childClosed := false

	c := di.New()
	c.Provide(func() (*A, func(), error) {
		return &A{}, func() { parentClosed = true }, nil
	})
	c.Provide(func(a *A) (*B, func(), error) {
		return &B{A: a}, func() { childClosed = true }, nil
	}, di.WithLifetime(di.Scoped))

	// Resolve parent singleton (A) in parent container.
	var a *A
	c.Resolve(&a)

	// Resolve scoped B in a child scope.
	scope := c.Scope()
	var b *B
	scope.Resolve(&b)

	// Close only the child scope.
	scope.Close()
	if !childClosed {
		t.Error("expected child cleanup to run")
	}
	if parentClosed {
		t.Error("parent cleanup must not run when child scope closes")
	}
}

func TestCycleDetection(t *testing.T) {
	type X struct{}
	type Y struct{}

	c := di.New()
	// X depends on Y, Y depends on X → cycle
	c.Provide(func(*Y) *X { return &X{} })
	c.Provide(func(*X) *Y { return &Y{} })

	var x *X
	err := c.Resolve(&x)
	if err == nil {
		t.Fatal("expected cycle error, got nil")
	}
	if !strings.Contains(err.Error(), "cycle detected") {
		t.Errorf("expected 'cycle detected' in error, got: %v", err)
	}
}

func TestMissingProvider(t *testing.T) {
	c := di.New()
	var a *A
	err := c.Resolve(&a)
	if err == nil {
		t.Fatal("expected error for missing provider")
	}
	if !strings.Contains(err.Error(), "no provider") {
		t.Errorf("expected 'no provider' in error, got: %v", err)
	}
}

func TestAlias(t *testing.T) {
	c := di.New()
	c.Provide(newMyImpl)
	c.Alias((*MyIface)(nil), (**MyImpl)(nil))

	var iface MyIface
	if err := c.Resolve(&iface); err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if iface.Hello() != "hello" {
		t.Errorf("unexpected: %s", iface.Hello())
	}

	// Both the concrete and the interface should resolve to the same underlying instance.
	var impl *MyImpl
	c.Resolve(&impl)
	if impl != iface.(*MyImpl) {
		t.Error("expected Alias to share the same singleton as the concrete provider")
	}
}


func TestGraph(t *testing.T) {
	c := di.New()
	c.Provide(newA)
	c.Provide(newB)
	c.Provide(newC)

	dot := c.Graph()
	if !strings.Contains(dot, "digraph di") {
		t.Errorf("expected DOT header, got:\n%s", dot)
	}
	if !strings.Contains(dot, "->") {
		t.Errorf("expected edges in graph, got:\n%s", dot)
	}
}
