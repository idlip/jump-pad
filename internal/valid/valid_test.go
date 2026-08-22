package valid

import (
	"errors"
	"testing"
	"time"

	"jump-pad/internal/api"
)

func TestSlug(t *testing.T) {
	cases := []struct {
		in      string
		want    string
		wantErr bool
	}{
		{"MyLink", "mylink", false},
		{"my-link_2", "my-link_2", false},
		{"has space", "", true},
		{"has/slash", "", true},
		{"pastes", "", true},
		{"admin", "", true},
		{"", "", true},
	}

	for _, one := range cases {
		got, err := Slug(one.in)
		if one.wantErr {
			if err == nil {
				t.Errorf("Slug(%q): want an error, got nil", one.in)
			} else if !errors.Is(err, api.ErrInvalid) {
				t.Errorf("Slug(%q) error = %v, want it to wrap api.ErrInvalid", one.in, err)
			}
			continue
		}
		if err != nil {
			t.Errorf("Slug(%q): unexpected error: %v", one.in, err)
			continue
		}
		if got != one.want {
			t.Errorf("Slug(%q) = %q, want %q", one.in, got, one.want)
		}
	}
}

func TestTargetURL(t *testing.T) {
	cases := []struct {
		in      string
		wantErr bool
	}{
		{"https://example.com", false},
		{"http://localhost:8080/x", false},
		{"https://192.168.1.1", false},
		{"https://idlip", true},       // no dot, not an IP, not localhost
		{"javascript:alert(1)", true}, // not http or https
		{"not a url", true},
		{"", true},
	}

	for _, one := range cases {
		_, err := TargetURL(one.in)
		if one.wantErr && err == nil {
			t.Errorf("TargetURL(%q): want an error, got nil", one.in)
		}
		if !one.wantErr && err != nil {
			t.Errorf("TargetURL(%q): unexpected error: %v", one.in, err)
		}
	}
}

func TestExpiry(t *testing.T) {
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)

	if at, err := Expiry("", now); err != nil || at != nil {
		t.Fatalf(`Expiry("") = %v, %v, want nil, nil`, at, err)
	}

	got, err := Expiry("2027-06-15", now)
	if err != nil {
		t.Fatalf("Expiry: %v", err)
	}
	want := time.Date(2027, 6, 15, 0, 0, 0, 0, time.UTC).Unix()
	if got == nil || *got != want {
		t.Fatalf("Expiry(2027-06-15) = %v, want %d", got, want)
	}

	if _, err := Expiry("2020-01-01", now); err == nil {
		t.Fatal("Expiry with a past date: want an error, got nil")
	}

	if at, err := Expiry("72h", now); err != nil || at == nil || *at != now.Add(72*time.Hour).Unix() {
		t.Fatalf(`Expiry("72h") = %v, %v`, at, err)
	}
}

func TestReserve(t *testing.T) {
	if IsReserved("coupons") {
		t.Fatal("coupons is reserved before Reserve ran")
	}
	Reserve("coupons")
	if !IsReserved("coupons") {
		t.Fatal("coupons is not reserved after Reserve ran")
	}
	if _, err := Slug("coupons"); err == nil {
		t.Fatal("Slug(coupons): want an error after Reserve ran")
	}
}
