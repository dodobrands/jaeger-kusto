package store

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/Azure/azure-kusto-go/azkustodata/value"
	"github.com/hashicorp/go-hclog"
	"github.com/jaegertracing/jaeger/model"
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

// A null attribute value (an unset OTLP AnyValue) must not fail the whole span.
// Reproduces "Error parsing span to domain. Error invalid tag type in <nil>".
func TestTransformKustoSpanToModelSpanNullTag(t *testing.T) {
	logger := hclog.Default()

	span := &kustoSpan{
		TraceID:            "85b50c451e131465fa1554dc91e4c553",
		SpanID:             "c16c6e4df0c111a7",
		SpanName:           "isMaster",
		ProcessServiceName: "mapi-internal-api",
		References:         value.Dynamic{Value: []byte("[]")},
		Logs:               value.Dynamic{Value: []byte("[]")},
		Tags: value.Dynamic{
			Value: []byte(`{"db.system":"mongodb","db.collection.name":null,"db.namespace":"admin"}`),
		},
		ProcessTags: value.Dynamic{
			Value: []byte(`{"service.name":"mapi-internal-api","k8s.node.zone":null}`),
		},
	}

	modelSpan, err := transformKustoSpanToModelSpan(span, logger)
	if err != nil {
		t.Fatal("transform failed on null tag:", err)
	}

	assertEmptyStringTag(t, modelSpan.Tags, "db.collection.name")
	assertEmptyStringTag(t, modelSpan.Process.Tags, "k8s.node.zone")
}

func assertEmptyStringTag(t *testing.T, tags []model.KeyValue, key string) {
	t.Helper()
	for _, tag := range tags {
		if tag.Key != key {
			continue
		}
		if tag.VType != model.StringType || tag.VStr != "" {
			t.Fatalf("tag %s = %v (%v), want empty string", key, tag.VStr, tag.VType)
		}
		return
	}
	t.Fatalf("tag %s missing from %v", key, tags)
}
