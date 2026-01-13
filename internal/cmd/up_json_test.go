package cmd

import (
	"bytes"
	"encoding/json"
	"testing"
)

func TestBuildUpSummary(t *testing.T) {
	services := []ServiceStatus{
		{Name: "Daemon", Type: "daemon", OK: true, Detail: "PID 123"},
		{Name: "Deacon", Type: "deacon", OK: true, Detail: "gt-deacon"},
		{Name: "Mayor", Type: "mayor", OK: false, Detail: "failed"},
	}

	summary := buildUpSummary(services)
	if summary.Total != 3 {
		t.Fatalf("Total = %d, want 3", summary.Total)
	}
	if summary.Started != 2 {
		t.Fatalf("Started = %d, want 2", summary.Started)
	}
	if summary.Failed != 1 {
		t.Fatalf("Failed = %d, want 1", summary.Failed)
	}
}

func TestEmitUpJSON_Success(t *testing.T) {
	services := []ServiceStatus{
		{Name: "Daemon", Type: "daemon", OK: true, Detail: "PID 123"},
		{Name: "Deacon", Type: "deacon", OK: true, Detail: "gt-deacon"},
	}

	var buf bytes.Buffer
	err := emitUpJSON(&buf, true, services)
	if err != nil {
		t.Fatalf("emitUpJSON returned error: %v", err)
	}

	var output UpOutput
	if err := json.Unmarshal(buf.Bytes(), &output); err != nil {
		t.Fatalf("invalid JSON output: %v\noutput: %s", err, buf.String())
	}

	if !output.Success {
		t.Fatalf("Success = %v, want true", output.Success)
	}
	if len(output.Services) != 2 {
		t.Fatalf("len(Services) = %d, want 2", len(output.Services))
	}
	if output.Summary.Total != 2 || output.Summary.Started != 2 || output.Summary.Failed != 0 {
		t.Fatalf("unexpected summary: %+v", output.Summary)
	}
}

func TestEmitUpJSON_FailureReturnsSilentExitAndValidJSON(t *testing.T) {
	services := []ServiceStatus{
		{Name: "Daemon", Type: "daemon", OK: true, Detail: "PID 123"},
		{Name: "Mayor", Type: "mayor", OK: false, Detail: "start failed"},
	}

	var buf bytes.Buffer
	err := emitUpJSON(&buf, false, services)
	if err == nil {
		t.Fatal("emitUpJSON should return error when allOK=false")
	}
	code, ok := IsSilentExit(err)
	if !ok {
		t.Fatalf("expected SilentExitError, got: %T (%v)", err, err)
	}
	if code != 1 {
		t.Fatalf("silent exit code = %d, want 1", code)
	}

	var output UpOutput
	if err := json.Unmarshal(buf.Bytes(), &output); err != nil {
		t.Fatalf("invalid JSON output: %v\noutput: %s", err, buf.String())
	}

	if output.Success {
		t.Fatalf("Success = %v, want false", output.Success)
	}
	if output.Summary.Total != 2 || output.Summary.Started != 1 || output.Summary.Failed != 1 {
		t.Fatalf("unexpected summary: %+v", output.Summary)
	}
}
