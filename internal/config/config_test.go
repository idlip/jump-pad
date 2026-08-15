package config

import "testing"

func TestWithFile(t *testing.T) {
	data := `
# a comment, skipped

addr = :9090
db_path=custom.db
auth_token = secret123
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
		RedirectPrefix: "/r/",
		PastePrefix:    "/p/",
	}
	if cfg != want {
		t.Fatalf("cfg = %+v, want %+v", cfg, want)
	}
}
