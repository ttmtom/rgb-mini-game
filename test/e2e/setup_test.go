//go:build e2e

package e2e_test

import (
	"context"
	"log"
	"os"
	"testing"

	"rgb-game/test/e2e/harness"
)

var h *harness.Harness

func TestMain(m *testing.M) {
	ctx := context.Background()
	var cleanup func()
	var err error
	h, cleanup, err = harness.New(ctx)
	if err != nil {
		log.Fatalf("harness setup failed: %v", err)
	}
	code := m.Run()
	cleanup()
	os.Exit(code)
}
