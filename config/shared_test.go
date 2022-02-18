package config

import (
	"github.com/stretchr/testify/assert"
	"os"
	"testing"
)

func Test_ToEnvironmentVariable(testing *testing.T) {
	name := "DiagnosticsProfilingEnabled"

	assert.Equal(testing, "DIAGNOSTICS_PROFILING_ENABLED", toEnvironmentVariable(name))
}

func Test_Override(testing *testing.T) {
	data := &struct {
		KeyOne string
		KeyTwo string
	}{
		KeyOne: "initial",
		KeyTwo: "initial",
	}

	err := os.Setenv("TESTING_KEY_ONE", "override")
	if err != nil {
		testing.Fatal(err)
	}
	if err := override("TESTING", data); err != nil {
		testing.Fatal(err)
	}

	assert.Equal(testing, "override", data.KeyOne)
	assert.Equal(testing, "initial", data.KeyTwo)
}
