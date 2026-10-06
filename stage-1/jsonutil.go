package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"math"
	"net/http"
	"regexp"
	"strconv"
	"unicode/utf8"
)

const (
	maxAmount  int64 = 1_000_000_000
	maxBalance int64 = 1 << 53
	maxBody          = 1 << 20
)

var (
	digits   = regexp.MustCompile(`^[0-9]+$`)
	handleRe = regexp.MustCompile(`^[a-z0-9_]{1,20}$`)
)

type apiError struct {
	Status int
	Code   string
	Msg    string
}

func (e *apiError) write(w http.ResponseWriter) {
	if e == nil {
		return
	}
	writeErr(w, e.Status, e.Code, e.Msg)
}

func malformed(msg string) *apiError {
	return &apiError{Status: http.StatusBadRequest, Code: "malformed_request", Msg: msg}
}

func validation(msg string) *apiError {
	return &apiError{Status: http.StatusUnprocessableEntity, Code: "validation_failed", Msg: msg}
}

func funds() *apiError {
	return &apiError{Status: http.StatusConflict, Code: "insufficient_funds", Msg: "insufficient funds"}
}

func writeErr(w http.ResponseWriter, status int, code, msg string) {
	writeJSON(w, status, map[string]any{
		"error": map[string]string{"code": code, "message": msg},
	})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	body, err := marshalJSON(v)
	if err != nil {
		http.Error(w, `{"error":{"code":"internal","message":"encode failed"}}`, http.StatusInternalServerError)
		return
	}
	writeRaw(w, status, body)
}

func writeRaw(w http.ResponseWriter, status int, body []byte) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_, _ = w.Write(body)
}

func writeEmpty(w http.ResponseWriter) {
	w.WriteHeader(http.StatusNoContent)
}

func marshalJSON(v any) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	return bytes.TrimRight(buf.Bytes(), "\n"), nil
}

func readBody(r *http.Request) ([]byte, error) {
	defer r.Body.Close()
	b, err := io.ReadAll(io.LimitReader(r.Body, maxBody+1))
	if err != nil {
		return nil, err
	}
	if len(b) > maxBody {
		return nil, errors.New("body too large")
	}
	return b, nil
}

func parseAny(body []byte) (any, error) {
	dec := json.NewDecoder(bytes.NewReader(body))
	dec.UseNumber()
	var v any
	if err := dec.Decode(&v); err != nil {
		return nil, err
	}
	var extra any
	if err := dec.Decode(&extra); err != io.EOF {
		return nil, errors.New("trailing json")
	}
	return v, nil
}

func parseObject(body []byte, emptyOK bool) (map[string]any, error) {
	if len(bytes.TrimSpace(body)) == 0 {
		if emptyOK {
			return map[string]any{}, nil
		}
		return nil, errors.New("empty body")
	}
	v, err := parseAny(body)
	if err != nil {
		return nil, err
	}
	obj, ok := v.(map[string]any)
	if !ok {
		return nil, errors.New("not an object")
	}
	return obj, nil
}

func canonical(v any) ([]byte, error) {
	return json.Marshal(norm(v))
}

func recanon(raw []byte) ([]byte, error) {
	v, err := parseAny(raw)
	if err != nil {
		return nil, err
	}
	return canonical(v)
}

func norm(v any) any {
	switch t := v.(type) {
	case map[string]any:
		m := make(map[string]any, len(t))
		for k, val := range t {
			m[k] = norm(val)
		}
		return m
	case []any:
		a := make([]any, len(t))
		for i := range t {
			a[i] = norm(t[i])
		}
		return a
	case json.Number:
		n, ok := parseIntegral(t)
		if ok {
			return n
		}
		f, err := t.Float64()
		if err != nil || math.IsNaN(f) || math.IsInf(f, 0) {
			return t.String()
		}
		return f
	default:
		return v
	}
}

func parseIntegral(v any) (int64, bool) {
	n, ok := v.(json.Number)
	if !ok {
		return 0, false
	}
	f, err := n.Float64()
	if err != nil || math.IsNaN(f) || math.IsInf(f, 0) || f != math.Trunc(f) {
		return 0, false
	}
	if math.Abs(f) > float64(maxBalance) {
		return 0, false
	}
	return int64(f), true
}

func parseAmount(v any) (int64, bool) {
	n, ok := parseIntegral(v)
	if !ok || n < 1 || n > maxAmount {
		return 0, false
	}
	return n, true
}

func requireAmount(obj map[string]any) (int64, *apiError) {
	v, ok := obj["amount"]
	if !ok {
		return 0, validation("amount is required")
	}
	switch v.(type) {
	case json.Number:
		n, ok := parseAmount(v)
		if !ok {
			return 0, validation("amount must be an integer from 1 to 1000000000")
		}
		return n, nil
	case string, bool:
		return 0, validation("amount must be an integer from 1 to 1000000000")
	default:
		return 0, malformed("amount has the wrong type")
	}
}

func optionalNote(obj map[string]any) (string, *apiError) {
	v, ok := obj["note"]
	if !ok {
		return "", nil
	}
	s, ok := v.(string)
	if !ok {
		return "", validation("note must be a string")
	}
	if utf8.RuneCountInString(s) > 200 {
		return "", validation("note must be at most 200 characters")
	}
	return s, nil
}

func optionalVisibility(obj map[string]any) (string, *apiError) {
	v, ok := obj["visibility"]
	if !ok {
		return "public", nil
	}
	s, ok := v.(string)
	if !ok || (s != "public" && s != "private") {
		return "", validation("visibility must be public or private")
	}
	return s, nil
}

func requireString(obj map[string]any, field string) (string, *apiError) {
	v, ok := obj[field]
	if !ok {
		return "", validation(field + " is required")
	}
	s, ok := v.(string)
	if !ok {
		return "", malformed(field + " must be a string")
	}
	return s, nil
}

func parsePage(r *http.Request) (limit int, offset int, huge bool, err *apiError) {
	q := r.URL.Query()
	limit = 50
	if vals, ok := q["limit"]; ok {
		if len(vals) == 0 || !digits.MatchString(vals[0]) {
			return 0, 0, false, validation("limit must be an integer from 1 to 200")
		}
		n, convErr := strconv.ParseInt(vals[0], 10, 64)
		if convErr != nil || n < 1 || n > 200 {
			return 0, 0, false, validation("limit must be an integer from 1 to 200")
		}
		limit = int(n)
	}
	if vals, ok := q["offset"]; ok {
		if len(vals) == 0 || !digits.MatchString(vals[0]) {
			return 0, 0, false, validation("offset must be an integer of 0 or more")
		}
		n, convErr := strconv.ParseInt(vals[0], 10, 64)
		if convErr != nil || n > int64(^uint(0)>>1) {
			return limit, 0, true, nil
		}
		offset = int(n)
	}
	return limit, offset, false, nil
}

func pageSlice[T any](items []T, offset, limit int, huge bool) ([]T, bool) {
	if huge || offset >= len(items) {
		return []T{}, false
	}
	end := offset + limit
	more := end < len(items)
	if end > len(items) {
		end = len(items)
	}
	out := append([]T{}, items[offset:end]...)
	return out, more
}
