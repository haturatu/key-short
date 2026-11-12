package desktop

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

func Read(path string) (Application, error) {
	if !strings.EqualFold(filepath.Ext(path), ".app") {
		return Application{}, fmt.Errorf(".app を選択してください")
	}
	data, err := exec.Command("/usr/bin/plutil", "-convert", "json", "-o", "-", filepath.Join(path, "Contents/Info.plist")).Output()
	if err != nil {
		return Application{}, fmt.Errorf("アプリ情報を読めません: %w", err)
	}
	var info map[string]any
	if err = json.Unmarshal(data, &info); err != nil {
		return Application{}, err
	}
	get := func(key string) string { s, _ := info[key].(string); return s }
	name := get("CFBundleDisplayName")
	if name == "" {
		name = get("CFBundleName")
	}
	if name == "" {
		name = strings.TrimSuffix(filepath.Base(path), ".app")
	}
	return Application{Name: name, Path: path, ID: get("CFBundleIdentifier"), Class: get("CFBundleIdentifier"), Exec: filepath.Join(path, "Contents/MacOS", get("CFBundleExecutable"))}, nil
}
func Command(a Application) ([]string, error) { return []string{"/usr/bin/open", "-a", a.Path}, nil }
func Applications() []Application {
	home, _ := os.UserHomeDir()
	var apps []Application
	seen := map[string]bool{}
	for _, root := range []string{filepath.Join(home, "Applications"), "/Applications", "/System/Applications"} {
		filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			if !d.IsDir() || !strings.HasSuffix(path, ".app") {
				return nil
			}
			a, err := Read(path)
			if err == nil && !seen[a.ID] {
				seen[a.ID] = true
				apps = append(apps, a)
			}
			return filepath.SkipDir
		})
	}
	sort.Slice(apps, func(i, j int) bool { return apps[i].Name < apps[j].Name })
	return apps
}
