package profile

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"

	"key-short/internal/protocol"
)

type Profile struct {
	Version  int                `json:"version"`
	Device   string             `json:"device"`
	LED      *int               `json:"led"`
	Bindings []protocol.Binding `json:"bindings"`
}

func New() Profile { return Profile{Version: 1, Device: "1189:8890", Bindings: []protocol.Binding{}} }

func (p Profile) Jobs() ([]protocol.Job, error) {
	if p.Version != 1 || p.Device != "1189:8890" {
		return nil, fmt.Errorf("version: 1、device: 1189:8890 の設定ファイルが必要です")
	}
	if len(p.Bindings) > 54 {
		return nil, fmt.Errorf("割り当ては最大54件です")
	}
	jobs := []protocol.Job{}
	seen := map[[2]int]bool{}
	for _, b := range p.Bindings {
		id := [2]int{b.Layer, b.Slot}
		if seen[id] {
			return nil, fmt.Errorf("レイヤー%d・入力%dが重複しています", b.Layer, b.Slot)
		}
		seen[id] = true
		frames, err := b.Frames()
		if err != nil {
			return nil, err
		}
		jobs = append(jobs, protocol.Job{Label: fmt.Sprintf("レイヤー %d / %s", b.Layer, protocol.SlotName(b.Slot)), Frames: frames})
	}
	if p.LED != nil {
		frames, err := protocol.LEDFrames(*p.LED)
		if err != nil {
			return nil, err
		}
		jobs = append(jobs, protocol.Job{Label: "LED", Frames: frames})
	}
	return jobs, nil
}

func (p Profile) Get(layer, slot int) (protocol.Binding, bool) {
	for _, b := range p.Bindings {
		if b.Layer == layer && b.Slot == slot {
			return b, true
		}
	}
	return protocol.Binding{}, false
}

func (p *Profile) Set(b protocol.Binding) {
	p.Remove(b.Layer, b.Slot)
	p.Bindings = append(p.Bindings, b)
	sort.Slice(p.Bindings, func(i, j int) bool {
		a, b := p.Bindings[i], p.Bindings[j]
		if a.Layer == b.Layer {
			return a.Slot < b.Slot
		}
		return a.Layer < b.Layer
	})
}

func (p *Profile) Remove(layer, slot int) {
	for i, b := range p.Bindings {
		if b.Layer == layer && b.Slot == slot {
			p.Bindings = append(p.Bindings[:i], p.Bindings[i+1:]...)
			return
		}
	}
}

func Decode(r io.Reader) (Profile, error) {
	var p Profile
	d := json.NewDecoder(io.LimitReader(r, 1024*1024))
	d.DisallowUnknownFields()
	if err := d.Decode(&p); err != nil {
		return p, fmt.Errorf("設定を読めません: %w", err)
	}
	if err := d.Decode(new(any)); err != io.EOF {
		return p, fmt.Errorf("設定ファイルの末尾に不要なデータがあります")
	}
	_, err := p.Jobs()
	return p, err
}

func Load(path string) (Profile, error) {
	f, err := os.Open(path)
	if err != nil {
		return Profile{}, err
	}
	defer f.Close()
	return Decode(f)
}

func (p Profile) Save(path string) error {
	if _, err := p.Jobs(); err != nil {
		return err
	}
	data, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return err
	}
	dir := filepath.Dir(path)
	if err = os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	f, err := os.CreateTemp(dir, ".key-short-*.json")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if _, err = f.Write(append(data, '\n')); err != nil {
		f.Close()
		return err
	}
	if err = f.Sync(); err != nil {
		f.Close()
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	return os.Rename(f.Name(), path)
}
