package desktop

import (
	"os"
	"path/filepath"
	"testing"
)

func TestXDGOverridesAndLocalizedNames(t *testing.T) {
	root := t.TempDir()
	user := filepath.Join(root, "user")
	system := filepath.Join(root, "system")
	for _, dir := range []string{user, system} {
		if err := os.MkdirAll(filepath.Join(dir, "applications"), 0700); err != nil {
			t.Fatal(err)
		}
	}
	write := func(dir, name, data string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, "applications", name), []byte(data), 0600); err != nil {
			t.Fatal(err)
		}
	}
	write(system, "hidden.desktop", "[Desktop Entry]\nType=Application\nName=System entry\nExec=ignored\n")
	write(user, "hidden.desktop", "[Desktop Entry]\nHidden=true\n")
	write(system, "debug.desktop", "[Desktop Entry]\nType=Application\nName=Browser\nName[ja]=ブラウザー\nStartupWMClass=debug-browser\nExec=chromium --remote-debugging-port=9222 %U\n")
	t.Setenv("XDG_DATA_HOME", user)
	t.Setenv("XDG_DATA_DIRS", system)
	t.Setenv("LANG", "ja_JP.UTF-8")
	t.Setenv("LC_MESSAGES", "")
	apps := Applications()
	var found bool
	for _, a := range apps {
		if a.ID == "hidden" {
			t.Fatal("hidden user override was ignored")
		}
		if a.ID == "debug" {
			found = true
			if a.Name != "ブラウザー" || a.Class != "debug-browser" {
				t.Fatalf("%+v", a)
			}
		}
	}
	if !found {
		t.Fatal("application missing")
	}
}
