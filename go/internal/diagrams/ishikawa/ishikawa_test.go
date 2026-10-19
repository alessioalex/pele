package ishikawa

import (
	"testing"

	"github.com/alessioalex/pele/go/internal/engine"
	"github.com/alessioalex/pele/go/internal/golden"
)

func TestGolden(t *testing.T) {
	golden.Run(t, "ishikawa", engine.NewRegistry(Diagram))
}
