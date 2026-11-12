package desktop

import (
	"encoding/json"
	"fmt"
	"golang.org/x/sys/windows"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

const shortcutScript = `$w=New-Object -ComObject WScript.Shell; $s=$w.CreateShortcut($env:KEY_SHORT_LINK); [pscustomobject]@{Name=[IO.Path]::GetFileNameWithoutExtension($env:KEY_SHORT_LINK);Path=$env:KEY_SHORT_LINK;Exec=$s.TargetPath;Arguments=$s.Arguments}|ConvertTo-Json -Compress`

func shortcut(path string) (Application, error) {
	cmd := exec.Command("powershell.exe", "-NoProfile", "-NonInteractive", "-Command", shortcutScript)
	cmd.Env = append(os.Environ(), "KEY_SHORT_LINK="+path)
	data, err := cmd.Output()
	if err != nil {
		return Application{}, fmt.Errorf("ショートカットを読めません: %w", err)
	}
	var value struct{ Name, Path, Exec, Arguments string }
	if err = json.Unmarshal(data, &value); err != nil {
		return Application{}, err
	}
	if value.Exec == "" {
		return Application{}, fmt.Errorf("実行ファイルを持たないショートカットです")
	}
	args, err := windows.DecomposeCommandLine(`"` + value.Exec + `" ` + value.Arguments)
	if err != nil {
		return Application{}, err
	}
	return Application{Name: value.Name, Path: path, Exec: value.Exec, ID: strings.TrimSuffix(strings.ToLower(filepath.Base(value.Exec)), ".exe"), Args: args}, nil
}
func Read(path string) (Application, error) {
	if _, err := os.Stat(path); err != nil {
		return Application{}, err
	}
	if strings.EqualFold(filepath.Ext(path), ".lnk") {
		return shortcut(path)
	}
	if !strings.EqualFold(filepath.Ext(path), ".exe") {
		return Application{}, fmt.Errorf(".lnk または .exe を選択してください")
	}
	return Application{Name: strings.TrimSuffix(filepath.Base(path), filepath.Ext(path)), ID: strings.TrimSuffix(strings.ToLower(filepath.Base(path)), ".exe"), Path: path, Exec: path, Args: []string{path}}, nil
}
func Command(a Application) ([]string, error) {
	if len(a.Args) == 0 {
		return nil, fmt.Errorf("起動コマンドがありません")
	}
	return a.Args, nil
}
func Applications() []Application {
	// Resolve all shortcuts in one PowerShell process rather than spawning a
	// process per menu item. Paths never become part of executable script text.
	script := `$w=New-Object -ComObject WScript.Shell; $roots=@([Environment]::GetFolderPath('StartMenu'),[Environment]::GetFolderPath('CommonStartMenu')); $r=@(foreach($root in $roots){Get-ChildItem -LiteralPath $root -Filter *.lnk -Recurse -ErrorAction SilentlyContinue|ForEach-Object {$s=$w.CreateShortcut($_.FullName);if($s.TargetPath -and [IO.Path]::GetExtension($s.TargetPath) -ieq '.exe'){[pscustomobject]@{Name=$_.BaseName;Path=$_.FullName;Exec=$s.TargetPath}}}});ConvertTo-Json -InputObject $r -Compress`
	data, err := exec.Command("powershell.exe", "-NoProfile", "-NonInteractive", "-Command", script).Output()
	if err != nil {
		return nil
	}
	var apps []Application
	if json.Unmarshal(data, &apps) != nil {
		return nil
	}
	for i := range apps {
		apps[i].ID = strings.TrimSuffix(strings.ToLower(filepath.Base(apps[i].Exec)), ".exe")
	}
	sort.Slice(apps, func(i, j int) bool { return strings.ToLower(apps[i].Name) < strings.ToLower(apps[j].Name) })
	return apps
}
