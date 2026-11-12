package device

import "testing"

func TestLayoutIdentityRequiresDurableLocation(t *testing.T) {
	if (Info{Bus: 2, Address: 3}).LayoutKey() != "" {
		t.Fatal("ephemeral bus address used for persistence")
	}
	a := Info{Serial: "same-factory-serial", Location: "2-1", Bus: 2, Address: 3}
	b := a
	b.Address = 9
	if a.LayoutKey() != b.LayoutKey() {
		t.Fatal("reconnect changed identity")
	}
	b.Location = "2-2"
	if a.LayoutKey() == b.LayoutKey() {
		t.Fatal("two ports with shared serial collide")
	}
}
