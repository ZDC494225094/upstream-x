package upstream

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestValidateSub2APILoginBodyTemplate(t *testing.T) {
	t.Parallel()

	if err := ValidateSub2APILoginBodyTemplate(`{"email":"{{email}}","password":"{{password}}"}`); err != nil {
		t.Fatalf("expected template to be valid, got %v", err)
	}
}

func TestValidateSub2APILoginBodyTemplateRejectsUnknownVariable(t *testing.T) {
	t.Parallel()

	err := ValidateSub2APILoginBodyTemplate(`{"account":"{{username}}","password":"{{password}}"}`)
	if err == nil {
		t.Fatal("expected validation error, got nil")
	}
	if !strings.Contains(err.Error(), "{{username}}") {
		t.Fatalf("expected unknown variable error, got %v", err)
	}
}

func TestRenderSub2APILoginBody(t *testing.T) {
	t.Parallel()

	body, err := RenderSub2APILoginBody(`{"account":"{{email}}","password":"{{password}}","meta":{"login_type":"email"}}`, "demo@example.com", `pa"ss`)
	if err != nil {
		t.Fatalf("render template: %v", err)
	}

	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatalf("decode rendered payload: %v", err)
	}

	if payload["account"] != "demo@example.com" {
		t.Fatalf("unexpected account value: %#v", payload["account"])
	}
	if payload["password"] != `pa"ss` {
		t.Fatalf("unexpected password value: %#v", payload["password"])
	}

	meta, ok := payload["meta"].(map[string]any)
	if !ok {
		t.Fatalf("unexpected meta payload: %#v", payload["meta"])
	}
	if meta["login_type"] != "email" {
		t.Fatalf("unexpected meta login_type: %#v", meta["login_type"])
	}
}

func TestRenderSub2APILoginBodyWithDefaultTemplate(t *testing.T) {
	t.Parallel()

	body, err := RenderSub2APILoginBody("", "demo@example.com", "secret")
	if err != nil {
		t.Fatalf("render default template: %v", err)
	}

	var payload map[string]string
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatalf("decode rendered payload: %v", err)
	}

	if payload["email"] != "demo@example.com" || payload["password"] != "secret" {
		t.Fatalf("unexpected default payload: %#v", payload)
	}
}
