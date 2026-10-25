package utils_test

import (
	"os"
	"testing"

	"rgb-game/pkg/utils"
)

// ─── GetEnv ──────────────────────────────────────────────────────────────────

func TestGetEnv_SetVar(t *testing.T) {
	const key = "TEST_GETENV_SET"
	t.Setenv(key, "hello")

	if got := utils.GetEnv(key); got != "hello" {
		t.Errorf("GetEnv(%q) = %q, want %q", key, got, "hello")
	}
}

func TestGetEnv_UnsetVar_NoFallback(t *testing.T) {
	const key = "TEST_GETENV_UNSET_NOFALLBACK"
	os.Unsetenv(key)

	if got := utils.GetEnv(key); got != "" {
		t.Errorf("GetEnv unset with no fallback want empty string, got %q", got)
	}
}

func TestGetEnv_UnsetVar_WithFallback(t *testing.T) {
	const key = "TEST_GETENV_UNSET_FALLBACK"
	os.Unsetenv(key)

	if got := utils.GetEnv(key, "default_val"); got != "default_val" {
		t.Errorf("GetEnv unset with fallback want %q, got %q", "default_val", got)
	}
}

func TestGetEnv_SetVar_FallbackIgnored(t *testing.T) {
	const key = "TEST_GETENV_SET_FALLBACK"
	t.Setenv(key, "actual")

	// Fallback must be ignored when the var is set.
	if got := utils.GetEnv(key, "ignored"); got != "actual" {
		t.Errorf("GetEnv set+fallback want %q, got %q", "actual", got)
	}
}

func TestGetEnv_EmptyStringValue(t *testing.T) {
	const key = "TEST_GETENV_EMPTY"
	t.Setenv(key, "")

	// An explicitly-set empty string should be returned as-is (not the fallback).
	if got := utils.GetEnv(key, "should_not_appear"); got != "" {
		t.Errorf("GetEnv empty value want empty string, got %q", got)
	}
}

// ─── Contains ────────────────────────────────────────────────────────────────

func TestContains_Found(t *testing.T) {
	if !utils.Contains([]string{"a", "b", "c"}, "b") {
		t.Error("Contains should return true when element is present")
	}
}

func TestContains_NotFound(t *testing.T) {
	if utils.Contains([]string{"a", "b", "c"}, "z") {
		t.Error("Contains should return false when element is absent")
	}
}

func TestContains_EmptySlice(t *testing.T) {
	if utils.Contains([]string{}, "a") {
		t.Error("Contains on empty slice should return false")
	}
}

func TestContains_NilSlice(t *testing.T) {
	if utils.Contains(nil, "a") {
		t.Error("Contains on nil slice should return false")
	}
}

func TestContains_FirstElement(t *testing.T) {
	if !utils.Contains([]string{"first", "second"}, "first") {
		t.Error("Contains should find the first element")
	}
}

func TestContains_LastElement(t *testing.T) {
	if !utils.Contains([]string{"first", "second", "last"}, "last") {
		t.Error("Contains should find the last element")
	}
}
