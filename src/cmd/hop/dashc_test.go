package main

import (
	"strings"
	"testing"
)

func TestExtractDashCFound(t *testing.T) {
	target, child, ok, err := extractDashC([]string{"hop", "-C", "outbox", "echo", "hi"})
	if !ok {
		t.Fatalf("expected ok=true")
	}
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if target != "outbox" {
		t.Errorf("target = %q, want outbox", target)
	}
	if len(child) != 2 || child[0] != "echo" || child[1] != "hi" {
		t.Errorf("child = %v, want [echo hi]", child)
	}
}

func TestExtractDashCEqualsForm(t *testing.T) {
	target, child, ok, err := extractDashC([]string{"hop", "-C=outbox", "echo"})
	if !ok || err != nil {
		t.Fatalf("ok=%v err=%v", ok, err)
	}
	if target != "outbox" {
		t.Errorf("target = %q", target)
	}
	if len(child) != 1 || child[0] != "echo" {
		t.Errorf("child = %v", child)
	}
}

func TestExtractDashCNoCommand(t *testing.T) {
	_, _, ok, err := extractDashC([]string{"hop", "-C", "outbox"})
	if !ok {
		t.Fatalf("expected ok=true (flag was found)")
	}
	if err == nil {
		t.Fatalf("expected err for missing child command")
	}
	if !strings.Contains(err.Error(), "command to execute") {
		t.Errorf("err: %v", err)
	}
}

func TestExtractDashCNoValue(t *testing.T) {
	_, _, ok, err := extractDashC([]string{"hop", "-C"})
	if !ok || err == nil {
		t.Fatalf("expected ok=true and err for missing value")
	}
	if !strings.Contains(err.Error(), "requires a value") {
		t.Errorf("err: %v", err)
	}
}

func TestExtractDashCAbsent(t *testing.T) {
	_, _, ok, _ := extractDashC([]string{"hop", "ls"})
	if ok {
		t.Fatalf("expected ok=false when -C absent")
	}
}
