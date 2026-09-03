package di

import (
	"fmt"
	"reflect"
	"strings"
)

// Graph returns a Graphviz DOT string representing the dependency graph of all
// registered providers. It introspects constructor argument types without
// resolving any instances, so it is safe to call before the container is used.
//
// Example output:
//
//	digraph di {
//	  rankdir=LR;
//	  "*service.LedgerService" -> "*repositories.PlayerRepository";
//	  "*repositories.PlayerRepository" -> "*gorm.DB";
//	}
func (c *Container) Graph() string {
	var sb strings.Builder
	sb.WriteString("digraph di {\n")
	sb.WriteString("  rankdir=LR;\n")

	// Collect all providers (including parent chain).
	providers := c.allProviders()

	for key, entry := range providers {
		ft := entry.fn.Type()
		src := dotLabel(key)
		for i := 0; i < ft.NumIn(); i++ {
			argKey := providerKey{typ: ft.In(i)}
			dst := dotLabel(argKey)
			fmt.Fprintf(&sb, "  %s -> %s;\n", quote(src), quote(dst))
		}
		// Isolated node (no deps) — still emit it so it appears in the graph.
		if ft.NumIn() == 0 {
			fmt.Fprintf(&sb, "  %s;\n", quote(src))
		}
	}

	sb.WriteString("}\n")
	return sb.String()
}

// allProviders collects providers from this container and all parents,
// with child providers taking precedence (child overrides parent).
func (c *Container) allProviders() map[providerKey]providerEntry {
	result := make(map[providerKey]providerEntry)
	if c.parent != nil {
		for k, v := range c.parent.allProviders() {
			result[k] = v
		}
	}
	for k, v := range c.providers {
		result[k] = v
	}
	return result
}

// dotLabel returns a human-readable label for a provider key.
func dotLabel(key providerKey) string {
	label := typeName(key.typ)
	if key.name != "" {
		label += "[" + key.name + "]"
	}
	return label
}

// typeName returns a compact, readable name for a reflect.Type.
func typeName(t reflect.Type) string {
	switch t.Kind() {
	case reflect.Ptr:
		return "*" + typeName(t.Elem())
	case reflect.Interface:
		return t.String()
	default:
		if t.PkgPath() == "" {
			return t.String()
		}
		// Use short package name + type name.
		parts := strings.Split(t.PkgPath(), "/")
		pkg := parts[len(parts)-1]
		return pkg + "." + t.Name()
	}
}

// quote wraps a string in double quotes for DOT output.
func quote(s string) string {
	return `"` + strings.ReplaceAll(s, `"`, `\"`) + `"`
}
