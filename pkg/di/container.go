// Package di provides a lightweight, reflection-based dependency injection container.
//
// Features:
//   - Named/tagged bindings (multiple providers per type)
//   - Three lifetimes: Singleton, Transient, Scoped
//   - Child scopes via Container.Scope()
//   - Cycle detection with descriptive error messages
//   - Interface aliases via ProvideAs
//   - LIFO cleanup via Container.Close()
//   - Dependency graph visualization via Container.Graph()
//
// Supported constructor shapes:
//
//	func(...args) T
//	func(...args) (T, error)
//	func(...args) (T, func(), error)   // func() is called by Close()
package di

import (
	"fmt"
	"reflect"
	"strings"
)

// providerKey identifies a binding by type and optional name tag.
type providerKey struct {
	typ  reflect.Type
	name string // "" for the default (unnamed) binding
}

func (k providerKey) String() string {
	if k.name == "" {
		return k.typ.String()
	}
	return fmt.Sprintf("%s[%s]", k.typ.String(), k.name)
}

// providerEntry holds a registered constructor and its configuration.
type providerEntry struct {
	fn       reflect.Value // the constructor func
	lifetime Lifetime
}

// cleanupEntry pairs a cleanup function with its provider key for diagnostics.
type cleanupEntry struct {
	key     providerKey
	cleanup func()
}

// Container is the DI container.
// The zero value is not usable; create one with New().
type Container struct {
	parent    *Container              // non-nil for child scopes
	providers map[providerKey]providerEntry
	instances map[providerKey]reflect.Value // singleton / scoped cache
	cleanups  []cleanupEntry                // LIFO on Close
}

// New returns a new, empty root Container.
func New() *Container {
	return &Container{
		providers: make(map[providerKey]providerEntry),
		instances: make(map[providerKey]reflect.Value),
	}
}

// Scope creates a child Container that shares the parent's provider registry and
// singleton instance cache. Scoped providers get their own per-child cache;
// child.Close() only drains the child's cleanup list.
func (c *Container) Scope() *Container {
	return &Container{
		parent:    c,
		providers: c.providers, // shared (read-only during resolution)
		instances: make(map[providerKey]reflect.Value),
	}
}

// ─── Registration ────────────────────────────────────────────────────────────

// Provide registers fn under its return type with the default (unnamed) binding.
func (c *Container) Provide(fn any, opts ...Option) *Container {
	return c.register("", fn, opts)
}

// ProvideNamed registers fn under its return type with the given name tag.
func (c *Container) ProvideNamed(name string, fn any, opts ...Option) *Container {
	return c.register(name, fn, opts)
}

// ProvideAs registers fn's concrete return type under the interface type pointed
// to by iface. iface must be a nil pointer to an interface, e.g. (*MyIface)(nil).
//
//	c.ProvideAs(repositories.NewPlayerRepository, (*out.PlayerRepository)(nil))
func (c *Container) ProvideAs(fn any, iface any, opts ...Option) *Container {
	ifaceType := reflect.TypeOf(iface)
	if ifaceType == nil || ifaceType.Kind() != reflect.Ptr || ifaceType.Elem().Kind() != reflect.Interface {
		panic("di: ProvideAs: iface must be a nil pointer to an interface, e.g. (*MyIface)(nil)")
	}
	return c.registerAs("", fn, ifaceType.Elem(), opts)
}

// ProvideAsNamed is like ProvideAs but registers under a name tag.
func (c *Container) ProvideAsNamed(name string, fn any, iface any, opts ...Option) *Container {
	ifaceType := reflect.TypeOf(iface)
	if ifaceType == nil || ifaceType.Kind() != reflect.Ptr || ifaceType.Elem().Kind() != reflect.Interface {
		panic("di: ProvideAsNamed: iface must be a nil pointer to an interface")
	}
	return c.registerAs(name, fn, ifaceType.Elem(), opts)
}

// Alias registers a zero-cost pass-through provider that resolves iface by
// looking up an already-registered concrete type. Neither a new constructor
// nor a new instance is created — the container reuses the singleton of the
// concrete type and boxes it into the interface.
//
//	c.Provide(service.NewLedgerService)                            // concrete
//	c.Alias((*in.LedgerUseCase)(nil), (**service.LedgerService)(nil)) // interface alias
//
// Both iface and concrete must be nil pointers to their respective types.
func (c *Container) Alias(iface any, concrete any) *Container {
	ifaceType := reflect.TypeOf(iface)
	concreteType := reflect.TypeOf(concrete)
	if ifaceType == nil || ifaceType.Kind() != reflect.Ptr || ifaceType.Elem().Kind() != reflect.Interface {
		panic("di: Alias: first argument must be a nil pointer to an interface")
	}
	if concreteType == nil || concreteType.Kind() != reflect.Ptr {
		panic("di: Alias: second argument must be a nil pointer to the concrete type")
	}
	ifaceElem := ifaceType.Elem()
	concreteElem := concreteType.Elem()

	funcType := reflect.FuncOf([]reflect.Type{concreteElem}, []reflect.Type{ifaceElem}, false)
	fn := reflect.MakeFunc(funcType, func(args []reflect.Value) []reflect.Value {
		result := reflect.New(ifaceElem).Elem()
		result.Set(args[0])
		return []reflect.Value{result}
	})

	c.providers[providerKey{typ: ifaceElem}] = providerEntry{fn: fn, lifetime: Singleton}
	return c
}

func (c *Container) register(name string, fn any, opts []Option) *Container {
	fv := reflect.ValueOf(fn)
	if fv.Kind() != reflect.Func {
		panic(fmt.Sprintf("di: Provide: expected a function, got %T", fn))
	}
	ft := fv.Type()
	retType := primaryReturnType(ft)
	return c.registerAs(name, fn, retType, opts)
}

func (c *Container) registerAs(name string, fn any, keyType reflect.Type, opts []Option) *Container {
	fv := reflect.ValueOf(fn)
	if fv.Kind() != reflect.Func {
		panic(fmt.Sprintf("di: Provide: expected a function, got %T", fn))
	}
	entry := providerEntry{fn: fv, lifetime: Singleton}
	for _, o := range opts {
		o(&entry)
	}
	key := providerKey{typ: keyType, name: name}
	c.providers[key] = entry
	return c
}

// ─── Resolution ──────────────────────────────────────────────────────────────

// Resolve fills *target with the resolved instance for target's element type.
// The target must be a non-nil pointer.
func (c *Container) Resolve(target any) error {
	return c.resolveNamed("", target)
}

// ResolveNamed resolves a named binding and fills *target.
func (c *Container) ResolveNamed(name string, target any) error {
	return c.resolveNamed(name, target)
}

// MustResolve is like Resolve but panics on error. Returns c for chaining.
func (c *Container) MustResolve(target any) *Container {
	if err := c.Resolve(target); err != nil {
		panic(fmt.Sprintf("di: MustResolve: %v", err))
	}
	return c
}

func (c *Container) resolveNamed(name string, target any) error {
	tv := reflect.ValueOf(target)
	if tv.Kind() != reflect.Ptr || tv.IsNil() {
		return fmt.Errorf("di: Resolve: target must be a non-nil pointer, got %T", target)
	}
	key := providerKey{typ: tv.Elem().Type(), name: name}
	val, err := c.resolve(key, nil)
	if err != nil {
		return err
	}
	tv.Elem().Set(val)
	return nil
}

// resolve is the recursive resolution engine.
// resolvingPath tracks the in-progress chain for cycle detection.
func (c *Container) resolve(key providerKey, resolvingPath []providerKey) (reflect.Value, error) {
	// Cycle detection.
	for _, k := range resolvingPath {
		if k == key {
			chain := make([]string, len(resolvingPath)+1)
			for i, k := range resolvingPath {
				chain[i] = k.String()
			}
			chain[len(resolvingPath)] = key.String()
			return reflect.Value{}, fmt.Errorf("di: cycle detected: %s", strings.Join(chain, " → "))
		}
	}

	entry, ok := c.lookupProvider(key)
	if !ok {
		return reflect.Value{}, fmt.Errorf("di: no provider registered for %s", key)
	}

	switch entry.lifetime {
	case Singleton:
		if val, hit := c.lookupSingletonInstance(key); hit {
			return val, nil
		}
	case Scoped:
		// Scoped instances are cached only in this container (not walked to parent).
		if val, hit := c.instances[key]; hit {
			return val, nil
		}
	case Transient:
		// Never cached; always construct fresh.
	}

	// Build arguments.
	path := append(resolvingPath, key)
	ft := entry.fn.Type()
	args := make([]reflect.Value, ft.NumIn())
	for i := 0; i < ft.NumIn(); i++ {
		argKey := providerKey{typ: ft.In(i)} // default name for arg deps
		val, err := c.resolve(argKey, path)
		if err != nil {
			return reflect.Value{}, err
		}
		args[i] = val
	}

	// Call the constructor.
	results := entry.fn.Call(args)
	instance, cleanup, err := extractResults(results)
	if err != nil {
		return reflect.Value{}, fmt.Errorf("di: constructor for %s failed: %w", key, err)
	}

	// Cache according to lifetime.
	switch entry.lifetime {
	case Singleton:
		c.storeSingleton(key, instance)
	case Scoped:
		c.instances[key] = instance
	}

	// Register cleanup (LIFO).
	if cleanup != nil {
		c.cleanups = append(c.cleanups, cleanupEntry{key: key, cleanup: cleanup})
	}

	return instance, nil
}

// lookupProvider walks up the parent chain.
func (c *Container) lookupProvider(key providerKey) (providerEntry, bool) {
	if entry, ok := c.providers[key]; ok {
		return entry, true
	}
	if c.parent != nil {
		return c.parent.lookupProvider(key)
	}
	return providerEntry{}, false
}

// lookupSingletonInstance walks up to find a cached singleton.
func (c *Container) lookupSingletonInstance(key providerKey) (reflect.Value, bool) {
	if val, ok := c.instances[key]; ok {
		return val, true
	}
	if c.parent != nil {
		return c.parent.lookupSingletonInstance(key)
	}
	return reflect.Value{}, false
}

// storeSingleton stores a singleton at the root level so all children share it.
func (c *Container) storeSingleton(key providerKey, val reflect.Value) {
	if c.parent != nil {
		c.parent.storeSingleton(key, val)
		return
	}
	c.instances[key] = val
}

// ─── Cleanup ─────────────────────────────────────────────────────────────────

// Close calls all registered cleanup functions in LIFO order.
// For child scopes, only the child's cleanups are called.
func (c *Container) Close() {
	for i := len(c.cleanups) - 1; i >= 0; i-- {
		c.cleanups[i].cleanup()
	}
	c.cleanups = nil
}

// ─── Helpers ─────────────────────────────────────────────────────────────────

// primaryReturnType returns the first return value's type of a function type.
// Panics if the function has no return values.
func primaryReturnType(ft reflect.Type) reflect.Type {
	if ft.NumOut() == 0 {
		panic("di: constructor must return at least one value")
	}
	return ft.Out(0)
}

// extractResults parses the return values of a constructor call into
// (instance, cleanupFn, error), supporting three shapes:
//
//	(T)
//	(T, error)
//	(T, func(), error)
func extractResults(results []reflect.Value) (reflect.Value, func(), error) {
	switch len(results) {
	case 1:
		return results[0], nil, nil

	case 2:
		// (T, error)
		instance := results[0]
		if !results[1].IsNil() {
			return reflect.Value{}, nil, results[1].Interface().(error)
		}
		return instance, nil, nil

	case 3:
		// (T, func(), error)
		instance := results[0]
		if !results[2].IsNil() {
			return reflect.Value{}, nil, results[2].Interface().(error)
		}
		var cleanup func()
		if !results[1].IsNil() {
			cleanup = results[1].Interface().(func())
		}
		return instance, cleanup, nil

	default:
		return reflect.Value{}, nil, fmt.Errorf("unsupported constructor return arity %d", len(results))
	}
}
