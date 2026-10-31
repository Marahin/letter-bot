package web

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestWriteJSON_EncodesBodyWithStatusAndContentType(t *testing.T) {
	// given
	rec := httptest.NewRecorder()

	// when
	WriteJSON(rec, http.StatusCreated, map[string]string{"url": "/images/site/7"})

	// then
	assert.Equal(t, http.StatusCreated, rec.Code)
	assert.Equal(t, "application/json; charset=utf-8", rec.Header().Get("Content-Type"))
	assert.JSONEq(t, `{"url":"/images/site/7"}`, rec.Body.String())
}

func TestWriteJSON_UnencodableValueYields500WithoutPartialBody(t *testing.T) {
	// given a value json cannot encode (a channel)
	rec := httptest.NewRecorder()

	// when
	WriteJSON(rec, http.StatusOK, map[string]any{"ch": make(chan int)})

	// then the failure surfaces as a 500 rather than a truncated body under a 200
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
	assert.NotContains(t, rec.Body.String(), `"ch"`)
}
