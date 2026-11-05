package main

import (
	"reflect"
	"testing"

	"GoRig/internal/base"
)

func TestOrganizationMenuEntries(t *testing.T) {
	got := organizationMenuEntries("http://127.0.0.1:8765/", []base.Organization{{ID: 12, Name: "A&B\tШкола"}, {ID: 25, Name: "Лицей"}})
	want := []organizationMenuEntry{
		{Label: "A&&B Школа", URL: "http://127.0.0.1:8765/#/organizations/12"},
		{Label: "Лицей", URL: "http://127.0.0.1:8765/#/organizations/25"},
		{Label: "Непривязанные", URL: "http://127.0.0.1:8765/#/organizations/0"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %#v, want %#v", got, want)
	}
}
