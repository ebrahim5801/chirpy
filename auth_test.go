package main

import (
	"net/http"
	"testing"
	"time"

	"ebrahim5801/chirpy/internal/auth"

	"github.com/google/uuid"
)

func TestMakeAndValidateJWT(t *testing.T) {
	userID := uuid.New()
	secret := "test-secret"

	token, err := auth.MakeJWT(userID, secret, time.Hour)
	if err != nil {
		t.Fatalf("MakeJWT: %v", err)
	}

	got, err := auth.ValidateJWT(token, secret)
	if err != nil {
		t.Fatalf("ValidateJWT: %v", err)
	}
	if got != userID {
		t.Errorf("got %v, want %v", got, userID)
	}
}

func TestValidateJWT_Expired(t *testing.T) {
	userID := uuid.New()
	secret := "test-secret"

	token, err := auth.MakeJWT(userID, secret, -time.Second)
	if err != nil {
		t.Fatalf("MakeJWT: %v", err)
	}

	_, err = auth.ValidateJWT(token, secret)
	if err == nil {
		t.Fatal("expected error for expired token, got nil")
	}
}

func TestValidateJWT_WrongSecret(t *testing.T) {
	userID := uuid.New()

	token, err := auth.MakeJWT(userID, "correct-secret", time.Hour)
	if err != nil {
		t.Fatalf("MakeJWT: %v", err)
	}

	_, err = auth.ValidateJWT(token, "wrong-secret")
	if err == nil {
		t.Fatal("expected error for wrong secret, got nil")
	}
}

func TestGetBearerToken(t *testing.T) {
	headers := http.Header{}
	headers.Set("Authorization", "Bearer my-token")

	token, err := auth.GetBearerToken(headers)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if token != "my-token" {
		t.Errorf("got %q, want %q", token, "my-token")
	}
}

func TestGetBearerToken_Missing(t *testing.T) {
	headers := http.Header{}

	_, err := auth.GetBearerToken(headers)
	if err == nil {
		t.Fatal("expected error for missing header, got nil")
	}
}

func TestGetBearerToken_InvalidFormat(t *testing.T) {
	headers := http.Header{}
	headers.Set("Authorization", "Token my-token")

	_, err := auth.GetBearerToken(headers)
	if err == nil {
		t.Fatal("expected error for invalid format, got nil")
	}
}
