package tui

import (
	"reflect"
	"testing"
)

// defaultServices wires a real implementation into every dependency, so no
// caller has to know which nil means "use the real one". waitForOllama is the
// one documented exception: its default needs probeOllama from the same value,
// see [services.waitForOllamaOrDefault].
func TestDefaultServices_wiresEveryDependency(t *testing.T) {
	t.Parallel()

	v := reflect.ValueOf(defaultServices())
	for i := range v.NumField() {
		name := v.Type().Field(i).Name
		if name == "waitForOllama" {
			continue
		}
		if f := v.Field(i); f.Kind() == reflect.Func && f.IsNil() {
			t.Errorf("defaultServices leaves %s nil", name)
		}
	}
}
