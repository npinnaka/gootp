package main

import "testing"

func TestPublicBaseURL(t *testing.T) {
    cases := []struct{
        in string
        want string
    }{
        {":8080", "http://localhost:8080"},
        {"0.0.0.0:8080", "http://localhost:8080"},
        {"[::]:8080", "http://localhost:8080"},
        {"127.0.0.1:8080", "http://127.0.0.1:8080"},
        {"example.com", "http://example.com"},
    }
    for _, c := range cases {
        got := publicBaseURL(c.in)
        if got != c.want {
            t.Fatalf("publicBaseURL(%q) = %q, want %q", c.in, got, c.want)
        }
    }
}
