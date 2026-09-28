package models

import (
	"testing"
)

func TestRuntimeInfo_Available_koboldCpp(t *testing.T) {
	t.Parallel()

	if !(RuntimeInfo{KoboldCppPath: "/d/koboldcpp"}).Available() {
		t.Error("Available() should be true when koboldcpp path is set")
	}
	if !(RuntimeInfo{KoboldCppRunning: true}).Available() {
		t.Error("Available() should be true when koboldcpp is running")
	}
	if !(RuntimeInfo{KoboldCppPath: "/d/koboldcpp", KoboldCppRunning: true}).Available() {
		t.Error("Available() should be true when koboldcpp path is set and running")
	}
	if (RuntimeInfo{}).Available() {
		t.Error("Available() should be false when no backends are present")
	}
}

func TestResolveKoboldCppPath(t *testing.T) {
	t.Parallel()

	p := ResolveKoboldCppPath(RuntimeInfo{KoboldCppPath: "/d/koboldcpp"})
	if p != "/d/koboldcpp" {
		t.Fatalf("got %q want /d/koboldcpp", p)
	}
	empty := ResolveKoboldCppPath(RuntimeInfo{})
	if empty != "" {
		t.Fatalf("got %q want empty", empty)
	}
}
