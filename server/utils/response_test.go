package utils

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/gin-gonic/gin"
	"net/http/httptest"
)

func setupTestContext() (*gin.Context, *httptest.ResponseRecorder) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	return c, w
}

func TestRespondErrorWithNil(t *testing.T) {
	c, w := setupTestContext()
	RespondError(c, 400, "InvalidParam", nil)

	if w.Code != 400 {
		t.Errorf("Status = %d, want 400", w.Code)
	}

	var body map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &body)
	if body["errid"] != "InvalidParam" {
		t.Errorf("errid = %v, want InvalidParam", body["errid"])
	}
	if body["message"] != "An internal error occurred" {
		t.Errorf("message = %v, want 'An internal error occurred'", body["message"])
	}
}

func TestRespondErrorWithError(t *testing.T) {
	c, w := setupTestContext()
	err := errors.New("test error")
	RespondError(c, 404, "NotFound", err)

	if w.Code != 404 {
		t.Errorf("Status = %d, want 404", w.Code)
	}

	var body map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &body)
	if body["errid"] != "NotFound" {
		t.Errorf("errid = %v, want NotFound", body["errid"])
	}
	if body["message"] != "An internal error occurred" {
		t.Errorf("message = %v, want 'An internal error occurred'", body["message"])
	}
}

func TestRespondSuccess(t *testing.T) {
	c, w := setupTestContext()
	RespondSuccess(c, gin.H{"key": "value"})

	if w.Code != 200 {
		t.Errorf("Status = %d, want 200", w.Code)
	}

	var body map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &body)
	if body["status"] != float64(200) {
		t.Errorf("status = %v, want 200", body["status"])
	}
	data, ok := body["data"].(map[string]interface{})
	if !ok {
		t.Fatal("data is not a map")
	}
	if data["key"] != "value" {
		t.Errorf("data[key] = %v, want value", data["key"])
	}
}

func TestRespondErrorStatusField(t *testing.T) {
	c, w := setupTestContext()
	RespondError(c, 500, "ServerError", errors.New("internal"))

	var body map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &body)
	if body["status"] != float64(500) {
		t.Errorf("status = %v, want 500", body["status"])
	}
}
