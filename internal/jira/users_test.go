package jira

import (
	"strings"
	"testing"

	"github.com/codewandler/dex/internal/render"
)

func TestIsAccountID(t *testing.T) {
	ids := []string{
		"5b10ac8d82e05b22cc7d4ef5",
		"557058:f58131cb-b67d-43c7-b30d-6b58d40bd077",
		"712020:0e9c1d5a-8a5b-4a2e-9d3c-1f2e3d4c5b6a",
	}
	for _, id := range ids {
		if !isAccountID(id) {
			t.Errorf("isAccountID(%q) = false, want true", id)
		}
	}
	notIDs := []string{"", "jane", "Jane Doe", "jane.doe@example.com", "5b10ac8d82e05b22cc7d4ef"}
	for _, s := range notIDs {
		if isAccountID(s) {
			t.Errorf("isAccountID(%q) = true, want false", s)
		}
	}
}

func TestOnlyPeople(t *testing.T) {
	users := []User{
		{AccountID: "a", AccountType: "atlassian", Active: true},
		{AccountID: "b", AccountType: "atlassian", Active: false},
		{AccountID: "c", AccountType: "app", Active: true},
		{AccountID: "d", AccountType: "customer", Active: true},
		{AccountID: "e", AccountType: "", Active: true},
	}
	got := onlyPeople(users)
	var ids []string
	for _, u := range got {
		ids = append(ids, u.AccountID)
	}
	if strings.Join(ids, ",") != "a,e" {
		t.Fatalf("onlyPeople kept %v, want [a e]", ids)
	}
}

func TestPickAccountID(t *testing.T) {
	jane := User{AccountID: "id-jane", DisplayName: "Jane Doe", EmailAddress: "jane@example.com"}
	janet := User{AccountID: "id-janet", DisplayName: "Janet Doe"}
	jane2 := User{AccountID: "id-jane2", DisplayName: "Jane Doe"}

	tests := []struct {
		name    string
		who     string
		people  []User
		want    string
		wantErr string
	}{
		{name: "single match", who: "jan", people: []User{janet}, want: "id-janet"},
		{name: "no match", who: "nobody", people: nil, wantErr: `no active user matches "nobody"`},
		{name: "exact name among several", who: "jane doe", people: []User{jane, janet}, want: "id-jane"},
		{name: "exact email among several", who: "JANE@example.com", people: []User{janet, jane}, want: "id-jane"},
		{name: "ambiguous partial", who: "jan", people: []User{jane, janet}, wantErr: "matches 2 users"},
		{name: "two exact name matches", who: "Jane Doe", people: []User{jane, jane2, janet}, wantErr: "matches 3 users"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := pickAccountID(tt.who, tt.people)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("err = %v, want containing %q", err, tt.wantErr)
				}
				for _, u := range tt.people {
					if !strings.Contains(err.Error(), u.AccountID) {
						t.Errorf("error %q does not list candidate %s", err, u.AccountID)
					}
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestUserListCompactEmptyPrintsNothing(t *testing.T) {
	ul := &UserList{}
	if got := ul.RenderText(render.ModeCompact); got != "" {
		t.Fatalf("compact output for no users = %q, want empty (scripts read it as account IDs)", got)
	}
}

func TestUserListCompactPrintsIDsOnly(t *testing.T) {
	ul := &UserList{Users: []User{{AccountID: "id-1", DisplayName: "A"}, {AccountID: "id-2", DisplayName: "B"}}}
	if got := ul.RenderText(render.ModeCompact); got != "id-1\nid-2\n" {
		t.Fatalf("compact output = %q", got)
	}
}
