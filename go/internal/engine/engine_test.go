package engine_test

import (
	"testing"

	"github.com/alessioalex/pele/go/internal/engine"
	"github.com/alessioalex/pele/go/internal/golden"
)

// Records that fail before a diagram type is known: limits, front matter, detection.
func TestGoldenUndetected(t *testing.T) {
	golden.Run(t, "_none", engine.NewRegistry())
}
