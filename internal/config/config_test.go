package config

import "testing"

func TestWithFile(t *testing.T) {
	data := `
# a comment, skipped

addr = :9090
db_path=custom.db
auth_token = secret123
admin_token = admin-token-0123456789
redirect_prefix = /r/
paste_prefix = /p/
`
	cfg, err := Defaults().WithFile(data)
	if err != nil {
		t.Fatalf("WithFile: %v", err)
	}

	want := Config{
		Addr:           ":9090",
		DBPath:         "custom.db",
		AuthToken:      "secret123",
		AdminToken:     "admin-token-0123456789",
		RedirectPrefix: "/r/",
		PastePrefix:    "/p/",
	}
	if cfg != want {
		t.Fatalf("cfg = %+v, want %+v", cfg, want)
	}
}

func TestValidate(t *testing.T) {
	cases := []struct {
		name    string
		cfg     Config
		wantErr bool
	}{
		{"defaults", Defaults(), false},
		{"an admin token that is long enough", Config{PastePrefix: "/pastes/", AdminToken: "admin-token-0123456789"}, false},
		{"the two prefixes resolve the same", Config{RedirectPrefix: "", PastePrefix: "/"}, true},
		{"the admin token is too short", Config{PastePrefix: "/pastes/", AdminToken: "short"}, true},
		{"the two tokens are equal", Config{PastePrefix: "/pastes/", AuthToken: "admin-token-0123456789", AdminToken: "admin-token-0123456789"}, true},
	}

	for _, one := range cases {
		err := one.cfg.Validate()
		if one.wantErr && err == nil {
			t.Errorf("%s: want an error, got nil", one.name)
		}
		if !one.wantErr && err != nil {
			t.Errorf("%s: unexpected error: %v", one.name, err)
		}
	}
}
