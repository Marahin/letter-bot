package web

import (
	"bytes"
	"encoding/json"
	"net/http"
)

// WriteJSON answers a JSON body with the given status. It encodes into a buffer
// before touching the header: streaming straight to w would land an encode failure
// as a truncated body under an already-sent 2xx.
func WriteJSON(w http.ResponseWriter, status int, v any) {
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(v); err != nil {
		http.Error(w, "encode response", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_, _ = buf.WriteTo(w)
}
