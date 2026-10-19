package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestLoadUsersTable(t *testing.T) {
	brokenPath := filepath.Join(t.TempDir(), "broken.xml")
	if err := os.WriteFile(brokenPath, []byte("<root><row>"), 0644); err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name    string
		path    string
		wantLen int
		wantErr bool
	}{
		{name: "ok", path: datasetPath, wantLen: 35},
		{name: "no file", path: "nope.xml", wantErr: true},
		{name: "file is broken", path: brokenPath, wantErr: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			users, err := loadUsers(tc.path)
			if (err != nil) != tc.wantErr {
				t.Fatalf("wantErr=%v, got err=%v", tc.wantErr, err)
			}
			if len(users) != tc.wantLen {
				t.Errorf("want %d users, got %d", tc.wantLen, len(users))
			}
		})
	}
}

func TestLoadUsersFirstUser(t *testing.T) {
	users, err := loadUsers(datasetPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(users) == 0 {
		t.Fatal("no users loaded")
	}

	want := User{
		ID:     0,
		Name:   "Boyd Wolf",
		Age:    22,
		About:  "Nulla cillum enim voluptate consequat laborum esse excepteur occaecat commodo nostrud excepteur ut cupidatat. Occaecat minim incididunt ut proident ad sint nostrud ad laborum sint pariatur. Ut nulla commodo dolore officia. Consequat anim eiusmod amet commodo eiusmod deserunt culpa. Ea sit dolore nostrud cillum proident nisi mollit est Lorem pariatur. Lorem aute officia deserunt dolor nisi aliqua consequat nulla nostrud ipsum irure id deserunt dolore. Minim reprehenderit nulla exercitation labore ipsum.\n",
		Gender: "male",
	}
	if users[0] != want {
		t.Errorf("first user:\nwant %+v\n got %+v", want, users[0])
	}
}

func TestSearchServerOK(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(SearchServer))
	defer ts.Close()

	sc := &SearchClient{
		AccessToken: validToken,
		URL: ts.URL,
	}

	resp, err := sc.FindUsers(SearchRequest{Limit: 10})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(resp.Users) != 35 {
		t.Errorf("want 35 users, got %d", len(resp.Users))
	}
}

func TestSearchServerBadToken(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(SearchServer))
	defer ts.Close()

	sc := &SearchClient{
		AccessToken: "invalidToken",
		URL: ts.URL,
	}

	_, err := sc.FindUsers(SearchRequest{Limit: 10})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	
	if err.Error() != "bad AccessToken" {
		t.Errorf("want err: bad AccessToken, got %v", err.Error())
	}
}

func TestSearchServerNoDataset(t *testing.T) {
	old := datasetPath
	datasetPath = "./unexistingWay.xml"
	ts := httptest.NewServer(http.HandlerFunc(SearchServer))
	defer func(){
		datasetPath = old
		ts.Close()
	}()

	sc := &SearchClient{
		AccessToken: validToken,
		URL: ts.URL,
	}

	_, err := sc.FindUsers(SearchRequest{})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	
	if err.Error() != "SearchServer fatal error" {
		t.Errorf("want err: SearchServer fatal error, got %v", err.Error())
	}
}