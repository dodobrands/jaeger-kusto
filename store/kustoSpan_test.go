package store

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/hashicorp/go-hclog"
)

func TestTransformReferencesToLinks(t *testing.T) {
	logger := hclog.Default()

	// Find the paths of all input files in the data directory.
	paths, err := filepath.Glob(filepath.Join("testdata", "*kustoSpanTests-1.txt"))
	if err != nil {
		t.Fatal(err)
	}

	for _, path := range paths {
		_, filename := filepath.Split(path)
		testname := filename[:len(filename)-len(filepath.Ext(path))]

		// Each path turns into a test: the test name is the filename without the
		// extension.
		t.Run(testname, func(t *testing.T) {
			source, err := os.ReadFile(path)
			if err != nil {
				t.Fatal("error reading source file:", err)
			}

			// Unmarshal the input file into a kustoSpan.
			var inputSpan kustoSpan
			_ = json.Unmarshal(source, &inputSpan)

			// >>> This is the actual code under test.
			_, errt := transformReferencesToLinks(&inputSpan, logger)
			if errt != nil {
				t.Fatal("error formatting:", err)
			}
		})
	}
}
