package main

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestNewLoggerJSON(t *testing.T) {
	var buf bytes.Buffer
	newLogger("json", &buf).Info("listening", "addr", ":8080")
	var line map[string]any
	if err := json.Unmarshal(buf.Bytes(), &line); err != nil {
		t.Fatalf("not JSON: %q: %v", buf.String(), err)
	}
	if line["msg"] != "listening" || line["addr"] != ":8080" || line["level"] != "INFO" {
		t.Errorf("got %v", line)
	}
}

func TestNewLoggerTextByDefault(t *testing.T) {
	var buf bytes.Buffer
	newLogger("", &buf).Info("listening", "addr", ":8080")
	if got := buf.String(); !strings.Contains(got, "msg=listening addr=:8080") {
		t.Errorf("got %q, want text output", got)
	}
}
