// Package desktop lists XDG application entries. gio handles the actual launch,
// including Exec field codes, D-Bus activation, and terminal applications.
package desktop

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

func Command(app Application) ([]string, error) { return []string{"gio", "launch", app.Path}, nil }

func Read(path string) (Application, error) {
	f, err := os.Open(path)
	if err != nil {
		return Application{}, err
	}
	defer f.Close()
	values := map[string]string{}
	section := ""
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "[") {
			section = line
			continue
		}
		if section != "[Desktop Entry]" || strings.HasPrefix(line, "#") {
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		if ok {
			values[k] = unescape(v)
		}
	}
	if err = scanner.Err(); err != nil {
		return Application{}, err
	}
	if values["Type"] != "Application" || values["Hidden"] == "true" {
		return Application{}, fmt.Errorf("起動できるアプリのエントリーではありません: %s", path)
	}
	name := values["Name"]
	loc := os.Getenv("LC_MESSAGES")
	if loc == "" {
		loc = os.Getenv("LANG")
	}
	loc = strings.Split(loc, ".")[0]
	for _, suffix := range []string{loc, strings.Split(loc, "_")[0]} {
		if n := values["Name["+suffix+"]"]; n != "" {
			name = n
			break
		}
	}
	if name == "" {
		name = filepath.Base(path)
	}
	return Application{Name: name, Path: path, ID: strings.TrimSuffix(filepath.Base(path), ".desktop"), Class: values["StartupWMClass"], Exec: values["Exec"]}, nil
}

func unescape(s string) string {
	return strings.NewReplacer(`\s`, " ", `\n`, "\n", `\t`, "\t", `\r`, "\r", `\\`, `\`).Replace(s)
}

func Applications() []Application {
	home, _ := os.UserHomeDir()
	dataHome := os.Getenv("XDG_DATA_HOME")
	if dataHome == "" {
		dataHome = filepath.Join(home, ".local/share")
	}
	dirs := os.Getenv("XDG_DATA_DIRS")
	if dirs == "" {
		dirs = "/usr/local/share:/usr/share"
	}
	roots := append([]string{dataHome}, filepath.SplitList(dirs)...)
	roots = append(roots, filepath.Join(home, ".local/share/flatpak/exports/share"), "/var/lib/flatpak/exports/share", "/var/lib/snapd/desktop")
	seen := map[string]bool{}
	var apps []Application
	for _, root := range roots {
		base := filepath.Join(root, "applications")
		filepath.WalkDir(base, func(path string, d os.DirEntry, err error) error {
			if err != nil || d.IsDir() || !strings.HasSuffix(path, ".desktop") {
				return nil
			}
			rel, _ := filepath.Rel(base, path)
			id := strings.ReplaceAll(rel, string(filepath.Separator), "-")
			if seen[id] {
				return nil
			}
			seen[id] = true
			app, err := Read(path)
			if err != nil {
				return nil
			}
			// NoDisplay entries are valid by explicit path, but not in the picker.
			data, _ := os.ReadFile(path)
			if strings.Contains("\n"+string(data), "\nNoDisplay=true") {
				return nil
			}
			app.ID = strings.TrimSuffix(id, ".desktop")
			apps = append(apps, app)
			return nil
		})
	}
	sort.Slice(apps, func(i, j int) bool { return strings.ToLower(apps[i].Name) < strings.ToLower(apps[j].Name) })
	return apps
}
