package handler

import (
    "bytes"
    "encoding/json"
    "net/http"
    "net/http/httptest"
    "testing"
    "time"

    miniredis "github.com/alicebob/miniredis/v2"
    "github.com/redis/go-redis/v9"
)

func newTestServer(t *testing.T) (*Server, func()) {
    t.Helper()
    mr, err := miniredis.Run()
    if err != nil {
        t.Fatalf("failed to start miniredis: %v", err)
    }
    rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
    s := New(rdb)
    cleanup := func() {
        _ = rdb.Close()
        mr.Close()
    }
    return s, cleanup
}

func doJSON(t *testing.T, handler http.HandlerFunc, method string, path string, body any) *httptest.ResponseRecorder {
    t.Helper()
    var buf bytes.Buffer
    if body != nil {
        if err := json.NewEncoder(&buf).Encode(body); err != nil {
            t.Fatalf("encode body: %v", err)
        }
    }
    req := httptest.NewRequest(method, path, &buf)
    req.Header.Set("Content-Type", "application/json")
    rr := httptest.NewRecorder()
    handler.ServeHTTP(rr, req)
    return rr
}

func TestGenerate_Success(t *testing.T) {
    s, cleanup := newTestServer(t)
    defer cleanup()

    rr := doJSON(t, s.HandleGenerate, http.MethodPost, "/generate", GenerateRequest{UserID: "u1"})
    if rr.Code != http.StatusOK {
        t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
    }
    var resp GenerateResponse
    if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
        t.Fatalf("decode resp: %v", err)
    }
    if resp.UserID != "u1" || len(resp.OTP) != 6 || resp.ExpiresIn != int((5*time.Minute).Seconds()) {
        t.Fatalf("unexpected resp: %+v", resp)
    }
}

func TestValidate_Flow_SuccessAndConsume(t *testing.T) {
    s, cleanup := newTestServer(t)
    defer cleanup()

    // Generate
    rr := doJSON(t, s.HandleGenerate, http.MethodPost, "/generate", GenerateRequest{UserID: "u1"})
    if rr.Code != http.StatusOK {
        t.Fatalf("generate expected 200, got %d: %s", rr.Code, rr.Body.String())
    }
    var gen GenerateResponse
    if err := json.NewDecoder(rr.Body).Decode(&gen); err != nil {
        t.Fatalf("decode gen: %v", err)
    }
    // Validate success
    rr2 := doJSON(t, s.HandleValidate, http.MethodPost, "/validate", ValidateRequest{UserID: "u1", OTP: gen.OTP})
    if rr2.Code != http.StatusOK {
        t.Fatalf("validate expected 200, got %d: %s", rr2.Code, rr2.Body.String())
    }
    var vresp ValidateResponse
    if err := json.NewDecoder(rr2.Body).Decode(&vresp); err != nil {
        t.Fatalf("decode vresp: %v", err)
    }
    if !vresp.Valid {
        t.Fatalf("expected valid=true, got %+v", vresp)
    }
    // Validate again should be expired/not found
    rr3 := doJSON(t, s.HandleValidate, http.MethodPost, "/validate", ValidateRequest{UserID: "u1", OTP: gen.OTP})
    if rr3.Code != http.StatusOK {
        t.Fatalf("validate2 expected 200, got %d: %s", rr3.Code, rr3.Body.String())
    }
    var vresp2 ValidateResponse
    if err := json.NewDecoder(rr3.Body).Decode(&vresp2); err != nil {
        t.Fatalf("decode vresp2: %v", err)
    }
    if vresp2.Valid {
        t.Fatalf("expected valid=false after consume, got %+v", vresp2)
    }
}

func TestValidate_WrongOTP(t *testing.T) {
    s, cleanup := newTestServer(t)
    defer cleanup()

    // Generate
    rr := doJSON(t, s.HandleGenerate, http.MethodPost, "/generate", GenerateRequest{UserID: "u1"})
    if rr.Code != http.StatusOK {
        t.Fatalf("generate expected 200, got %d", rr.Code)
    }
    // Wrong OTP
    rr2 := doJSON(t, s.HandleValidate, http.MethodPost, "/validate", ValidateRequest{UserID: "u1", OTP: "000000"})
    if rr2.Code != http.StatusOK {
        t.Fatalf("expected 200, got %d", rr2.Code)
    }
    var v ValidateResponse
    if err := json.NewDecoder(rr2.Body).Decode(&v); err != nil {
        t.Fatalf("decode: %v", err)
    }
    if v.Valid {
        t.Fatalf("expected invalid, got %+v", v)
    }
}

func TestHandlers_MethodNotAllowed_And_BadRequest(t *testing.T) {
    s, cleanup := newTestServer(t)
    defer cleanup()

    // Method not allowed
    rr := doJSON(t, s.HandleGenerate, http.MethodGet, "/generate", nil)
    if rr.Code != http.StatusMethodNotAllowed {
        t.Fatalf("expected 405, got %d", rr.Code)
    }

    // Bad request: missing fields
    rr2 := doJSON(t, s.HandleGenerate, http.MethodPost, "/generate", map[string]string{"foo": "bar"})
    if rr2.Code != http.StatusBadRequest {
        t.Fatalf("expected 400, got %d", rr2.Code)
    }

    rr3 := doJSON(t, s.HandleValidate, http.MethodPost, "/validate", map[string]string{"userId": "u1"})
    if rr3.Code != http.StatusBadRequest {
        t.Fatalf("expected 400, got %d", rr3.Code)
    }
}
