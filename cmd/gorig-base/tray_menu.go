package main

import (
	"strconv"
	"strings"
	"unicode"

	"GoRig/internal/base"
)

type organizationMenuEntry struct {
	Label string
	URL   string
}

// Labels use Win32 menu syntax, URLs contain numeric IDs rather than names.
func organizationMenuEntries(address string, organizations []base.Organization) []organizationMenuEntry {
	entries := make([]organizationMenuEntry, 0, len(organizations)+1)
	for _, org := range organizations {
		label := strings.Map(func(r rune) rune {
			if unicode.IsControl(r) {
				return ' '
			}
			return r
		}, org.Name)
		label = strings.ReplaceAll(label, "&", "&&")
		entries = append(entries, organizationMenuEntry{Label: label, URL: address + "#/organizations/" + strconv.FormatInt(org.ID, 10)})
	}
	return append(entries, organizationMenuEntry{Label: "Непривязанные", URL: address + "#/organizations/0"})
}
