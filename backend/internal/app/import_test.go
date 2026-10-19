package app

import (
	"strings"
	"testing"
)

func TestImportStablePrivateIDs(t *testing.T) {
	csv := "name,phone,email\nFamilia,+526561234567,\n"
	a, e := ReadGuests(strings.NewReader(csv), strings.Repeat("a", 32))
	if e != nil {
		t.Fatal(e)
	}
	b, _ := ReadGuests(strings.NewReader(csv), strings.Repeat("a", 32))
	c, _ := ReadGuests(strings.NewReader(csv), strings.Repeat("b", 32))
	if a[0].ID != b[0].ID || a[0].ID == c[0].ID {
		t.Fatal("IDs not stable and private")
	}
}
func TestImportRejectsDuplicates(t *testing.T) {
	_, e := ReadGuests(strings.NewReader("name,phone,email\nA,+526561234567,\nB,+526561234567,\n"), strings.Repeat("a", 32))
	if e == nil {
		t.Fatal("duplicate accepted")
	}
}
