package httpapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestReviewedPreferenceRegistryCoversCapturedTabsAndFields(t *testing.T) {
	registry := reviewedPreferenceRegistry()
	if len(registry) < 200 {
		t.Fatalf("reviewed preference registry has %d fields, want at least 200", len(registry))
	}
	seenCategories := make(map[string]bool)
	seenKeys := make(map[string]bool)
	seenStorageKeys := make(map[string]bool)
	for _, definition := range registry {
		seenCategories[definition.Category] = true
		key := definition.Category + "\x00" + definition.FieldKey
		if seenKeys[key] {
			t.Fatalf("duplicate preference field_key %q", key)
		}
		seenKeys[key] = true
		storageKey := definition.Category + "\x00" + definition.StorageCaption
		if seenStorageKeys[storageKey] {
			t.Fatalf("duplicate preference storage key %q", storageKey)
		}
		seenStorageKeys[storageKey] = true
		if definition.Caption == "" || definition.FieldKey == "" || definition.StorageCaption == "" || definition.Behavior == "" || definition.RuntimeStatus == "" {
			t.Fatalf("incomplete preference definition: %+v", definition)
		}
	}
	if len(seenCategories) != 17 {
		t.Fatalf("registry categories = %d, want 17", len(seenCategories))
	}
	schedule := preferenceDefinitionMap("Schedule")
	first, firstOK := schedule["schedule.activate.1"]
	second, secondOK := schedule["schedule.activate.2"]
	if !firstOK || !secondOK || first.FieldKey == second.FieldKey {
		t.Fatal("repeated Schedule Activate fields collapsed to one field key")
	}
}

func TestPreferenceValidationRejectsUnsafeTypedValues(t *testing.T) {
	definitions := preferenceDefinitionMap("General")
	if err := preferenceValidationError(definitions["Enable Alias Name:"], "maybe"); err == nil {
		t.Fatal("invalid boolean was accepted")
	}
	if err := preferenceValidationError(definitions["Max. Allowed Days:"], "not-a-number"); err == nil {
		t.Fatal("invalid integer was accepted")
	}
	if err := preferenceValidationError(definitions["Enable Alias Name:"], "Yes"); err != nil {
		t.Fatalf("valid boolean rejected: %v", err)
	}
}

func TestSchedulePreferenceIsExplicitlyNotConfigured(t *testing.T) {
	for _, definition := range reviewedPreferenceRegistry() {
		if definition.Category != "Schedule" {
			continue
		}
		if definition.RuntimeStatus != "not_configured" {
			t.Fatalf("schedule field %q status = %q", definition.Caption, definition.RuntimeStatus)
		}
		if !strings.Contains(strings.ToLower(definition.Behavior), "not configured") {
			t.Fatalf("schedule field %q does not document divergence: %q", definition.Caption, definition.Behavior)
		}
	}
	divergences := preferenceDivergences()
	if len(divergences) == 0 || divergences[0].Status != "not_configured" {
		t.Fatalf("schedule divergence contract = %+v", divergences)
	}
}

func TestPreferenceRuntimeStatusOnlyClaimsProvenBehavior(t *testing.T) {
	for _, definition := range reviewedPreferenceRegistry() {
		if definition.RuntimeStatus != "wired" {
			continue
		}
		if definition.Category != "Report" || definition.Caption != "Default Header On Report:" {
			t.Fatalf("unreviewed preference marked wired: %s/%s", definition.Category, definition.Caption)
		}
	}
}

func TestPreferencePermissionIsFailClosed(t *testing.T) {
	server := &Server{}
	request := preferenceTestRequest("GET", "/v1/preferences?category=General", &sessionContext{TenantID: "tenant-a"})
	recorder := httptest.NewRecorder()
	server.preferences(recorder, request)
	if recorder.Code != 403 {
		t.Fatalf("preference permission status = %d, want 403", recorder.Code)
	}
}

func preferenceTestRequest(method, target string, operator *sessionContext) *http.Request {
	return httptest.NewRequest(method, target, nil).WithContext(context.WithValue(context.Background(), sessionContextKey, operator))
}
