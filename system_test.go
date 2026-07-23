package main

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestParseMemory(t *testing.T) {
	input := `MemTotal:        5546856 kB
MemAvailable:    2783692 kB
SwapTotal:       6291452 kB
SwapFree:        4994044 kB`
	got, err := parseMemory([]byte(input))
	if err != nil {
		t.Fatal(err)
	}
	if got.TotalMB != 5416 || got.AvailableMB != 2718 || got.SwapUsedMB != 1267 || got.UsedPercent != 49.8 {
		t.Fatalf("unexpected memory values: %+v", got)
	}
}

func TestParseCPU(t *testing.T) {
	input := `Tasks: 2 total
800%cpu 0%user 800%idle
 PID USER PR NI VIRT RES SHR S[%CPU] %MEM TIME+ ARGS
 101 app 20 0 10G 4M 3M S 16.0 0.1 0:01 server

Tasks: 2 total
800%cpu 0%user 800%idle
 PID USER PR NI VIRT RES SHR S[%CPU] %MEM TIME+ ARGS
 101 app 20 0 10G 4M 3M S 6.0 0.1 0:02 server
 102 app 20 0 10G 4M 3M S 2.0 0.1 0:01 tunnel`
	got, err := parseCPU([]byte(input), 8)
	if err != nil {
		t.Fatal(err)
	}
	if got != 1 {
		t.Fatalf("usage = %v, want 1", got)
	}
}

func TestParseBattery(t *testing.T) {
	got, err := parseBattery([]byte(`{"health":"GOOD","status":"DISCHARGING","temperature":33.0,"percentage":90}`))
	if err != nil {
		t.Fatal(err)
	}
	if got.Percentage != 90 || got.Status != "DISCHARGING" || got.TemperatureCelsius != 33 {
		t.Fatalf("unexpected battery values: %+v", got)
	}
}

func TestParseStorage(t *testing.T) {
	input := "Filesystem 1K-blocks Used Available Use% Mounted on\n/dev/block/dm-79 113194976 12076432 100581676 11% /data/user/0"
	got, err := parseStorage([]byte(input))
	if err != nil {
		t.Fatal(err)
	}
	if got.TotalGB != 108 || got.AvailableGB != 95.9 || got.UsedPercent != 11 {
		t.Fatalf("unexpected storage values: %+v", got)
	}
}

func TestParsersRejectMalformedOutput(t *testing.T) {
	checks := []struct {
		name string
		fn   func() error
	}{
		{"memory", func() error { _, err := parseMemory([]byte("bad")); return err }},
		{"cpu", func() error { _, err := parseCPU([]byte("bad"), 8); return err }},
		{"battery", func() error { _, err := parseBattery([]byte("bad")); return err }},
		{"storage", func() error { _, err := parseStorage([]byte("bad")); return err }},
		{"uptime", func() error { _, err := parseUptime(nil); return err }},
	}
	for _, check := range checks {
		t.Run(check.name, func(t *testing.T) {
			if check.fn() == nil {
				t.Fatal("expected an error")
			}
		})
	}
}

func TestSystemHandlerReturnsDegradedWithoutSamples(t *testing.T) {
	c := newSystemCollector()
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/v1/system", nil)
	c.systemHandler(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status code = %d", recorder.Code)
	}
	var response SystemResponse
	if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
		t.Fatal(err)
	}
	if response.Status != "degraded" {
		t.Fatalf("status = %q, want degraded", response.Status)
	}
	if !strings.Contains(recorder.Header().Get("Cache-Control"), "no-store") {
		t.Fatal("expected no-store response")
	}
}

func TestCollectionFailureRetainsLastSuccessfulFastSample(t *testing.T) {
	c := newSystemCollector()
	c.cpu = CPUStats{Cores: 8, UsagePercent: 12.5}
	c.memory = MemoryStats{TotalMB: 5416, AvailableMB: 2700}
	c.fastHealthy = true
	c.run = func(string, ...string) ([]byte, error) {
		return nil, errors.New("unavailable")
	}

	c.collectFast()

	response := c.response()
	if response.Status != "degraded" {
		t.Fatalf("status = %q, want degraded", response.Status)
	}
	if response.CPU.UsagePercent != 12.5 || response.Memory.AvailableMB != 2700 {
		t.Fatalf("last successful sample was not retained: %+v", response)
	}
}

func TestSystemHandlerRejectsNonGET(t *testing.T) {
	c := newSystemCollector()
	recorder := httptest.NewRecorder()
	c.systemHandler(recorder, httptest.NewRequest(http.MethodPost, "/api/v1/system", nil))
	if recorder.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status code = %d, want %d", recorder.Code, http.StatusMethodNotAllowed)
	}
}
