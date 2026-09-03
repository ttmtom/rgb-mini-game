package di

// Lifetime controls how the container manages instances created by a provider.
type Lifetime int

const (
	// Singleton creates one instance per container and caches it. This is the default.
	Singleton Lifetime = iota
	// Transient creates a fresh instance on every Resolve call. Instances are never cached.
	Transient
	// Scoped creates one instance per child container (created via Container.Scope).
	// Parent singleton instances are shared across scopes.
	Scoped
)

// Option is a functional option for Provide/ProvideNamed/ProvideAs.
type Option func(*providerEntry)

// WithLifetime sets the lifetime for a provider.
func WithLifetime(l Lifetime) Option {
	return func(e *providerEntry) {
		e.lifetime = l
	}
}
