package models

import (
	"testing"
)

func TestBoolFromInt(t *testing.T) {
	if BoolFromInt(0) != false {
		t.Error("BoolFromInt(0) should be false")
	}
	if BoolFromInt(1) != true {
		t.Error("BoolFromInt(1) should be true")
	}
	if BoolFromInt(-1) != true {
		t.Error("BoolFromInt(-1) should be true")
	}
	if BoolFromInt(100) != true {
		t.Error("BoolFromInt(100) should be true")
	}
}

func TestBoolIntValue(t *testing.T) {
	var b BoolInt = true
	v, err := b.Value()
	if err != nil {
		t.Fatalf("Value() error: %v", err)
	}
	if v != true {
		t.Errorf("Value() = %v, want true", v)
	}

	b = false
	v, err = b.Value()
	if err != nil {
		t.Fatalf("Value() error: %v", err)
	}
	if v != false {
		t.Errorf("Value() = %v, want false", v)
	}
}

func TestBoolIntScan(t *testing.T) {
	tests := []struct {
		name     string
		input    interface{}
		expected BoolInt
	}{
		{"int64 zero", int64(0), false},
		{"int64 one", int64(1), true},
		{"int64 non-zero", int64(42), true},
		{"int zero", int(0), false},
		{"int one", int(1), true},
		{"bool true", true, true},
		{"bool false", false, false},
		{"nil", nil, false},
		{"string", "hello", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var b BoolInt
			err := b.Scan(tt.input)
			if err != nil {
				t.Fatalf("Scan(%v) error: %v", tt.input, err)
			}
			if b != tt.expected {
				t.Errorf("Scan(%v) = %v, want %v", tt.input, b, tt.expected)
			}
		})
	}
}

func TestBoolIntMarshalJSON(t *testing.T) {
	var b BoolInt = true
	data, err := b.MarshalJSON()
	if err != nil {
		t.Fatalf("MarshalJSON() error: %v", err)
	}
	if string(data) != "1" {
		t.Errorf("MarshalJSON() = %s, want 1", data)
	}

	b = false
	data, err = b.MarshalJSON()
	if err != nil {
		t.Fatalf("MarshalJSON() error: %v", err)
	}
	if string(data) != "0" {
		t.Errorf("MarshalJSON() = %s, want 0", data)
	}
}

func TestBoolIntUnmarshalJSON(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected BoolInt
		wantErr  bool
	}{
		{"zero", "0", false, false},
		{"one", "1", true, false},
		{"non-zero", "42", true, false},
		{"negative", "-1", true, false},
		{"invalid", "abc", false, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var b BoolInt
			err := b.UnmarshalJSON([]byte(tt.input))
			if tt.wantErr {
				if err == nil {
					t.Errorf("UnmarshalJSON(%s) expected error, got nil", tt.input)
				}
				return
			}
			if err != nil {
				t.Fatalf("UnmarshalJSON(%s) error: %v", tt.input, err)
			}
			if b != tt.expected {
				t.Errorf("UnmarshalJSON(%s) = %v, want %v", tt.input, b, tt.expected)
			}
		})
	}
}

func TestBoolIntJSONRoundTrip(t *testing.T) {
	original := BoolInt(true)
	data, err := original.MarshalJSON()
	if err != nil {
		t.Fatalf("MarshalJSON error: %v", err)
	}

	var decoded BoolInt
	err = decoded.UnmarshalJSON(data)
	if err != nil {
		t.Fatalf("UnmarshalJSON error: %v", err)
	}
	if decoded != original {
		t.Errorf("Round trip failed: %v -> %s -> %v", original, data, decoded)
	}
}
