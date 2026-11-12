package protocol

import (
	"reflect"
	"testing"
)

func TestWindowsCommandQuoting(t *testing.T) {
	args, err := CommandArgs(`"C:\Program Files\Chromium\chrome.exe" --remote-debugging-port=9222 --user-data-dir="C:\debug profile"`)
	want := []string{`C:\Program Files\Chromium\chrome.exe`, "--remote-debugging-port=9222", `--user-data-dir=C:\debug profile`}
	if err != nil || !reflect.DeepEqual(args, want) {
		t.Fatalf("%#v %v", args, err)
	}
}
