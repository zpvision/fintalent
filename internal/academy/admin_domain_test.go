package academy

import (
	"encoding/json"
	"testing"
)

func TestAdministrativeRolesAreSeparateAndLeastPrivilege(t *testing.T) {
	for _, test := range []struct {
		role, action string
		allowed      bool
	}{{"superadmin", "grants", true}, {"academy_admin", "edit", true}, {"academy_admin", "settings", false}, {"support", "access", true}, {"support", "recalculate", true}, {"support", "edit", false}, {"support", "transfer", false}, {"support", "correct", false}, {"moderator", "moderate", true}, {"moderator", "analytics", false}, {"moderator", "read", false}, {"moderator", "program", true}, {"", "read", false}} {
		p := AdminPrincipal{Key: "user:1", Role: test.role}
		if p.Can(test.action) != test.allowed {
			t.Fatalf("%s %s", test.role, test.action)
		}
	}
	p := AdminPrincipal{Key: "user:1", Role: "academy_admin"}
	if p.Can("correct") || p.Can("contacts") || p.Can("answers") {
		t.Fatal("sensitive permissions were implicit")
	}
	p.Correct = true
	p.Answers = true
	p.Contacts = true
	if !p.Can("correct") || !p.Can("answers") || !p.Can("contacts") {
		t.Fatal("explicit permissions ignored")
	}
}
func TestAdministrativeCommandsRequireConfirmationAndReason(t *testing.T) {
	c := AdminCommand{Reason: "small correction", Confirm: true, Key: Key(), Expected: adminHash(json.RawMessage(`{}`))}
	if err := c.validate(); err != nil {
		t.Fatal(err)
	}
	c.Confirm = false
	if c.validate() == nil {
		t.Fatal("unconfirmed change")
	}
	c.Confirm = true
	c.Reason = "x"
	if c.validate() == nil {
		t.Fatal("missing reason")
	}
	s := defaultAdminSettings()
	if s.validate() != nil {
		t.Fatal("defaults")
	}
	s.UploadMB = 21
	if s.validate() == nil {
		t.Fatal("unsafe upload limit")
	}
	s = defaultAdminSettings()
	s.ModerationRequired = false
	if s.validate() == nil {
		t.Fatal("inert moderation switch exposed")
	}
}
