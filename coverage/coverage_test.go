package main

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// boydWolf — первая запись из dataset.xml.
var boydWolf = User{
	ID:     0,
	Name:   "Boyd Wolf",
	Age:    22,
	About:  "Nulla cillum enim voluptate consequat laborum esse excepteur occaecat commodo nostrud excepteur ut cupidatat. Occaecat minim incididunt ut proident ad sint nostrud ad laborum sint pariatur. Ut nulla commodo dolore officia. Consequat anim eiusmod amet commodo eiusmod deserunt culpa. Ea sit dolore nostrud cillum proident nisi mollit est Lorem pariatur. Lorem aute officia deserunt dolor nisi aliqua consequat nulla nostrud ipsum irure id deserunt dolore. Minim reprehenderit nulla exercitation labore ipsum.\n",
	Gender: "male",
}

// newSearchServer поднимает тестовый сервер с SearchServer
// и гасит его по окончании теста.
func newSearchServer(t *testing.T) *httptest.Server {
	t.Helper()
	ts := httptest.NewServer(http.HandlerFunc(SearchServer))
	t.Cleanup(ts.Close)
	return ts
}

// setDatasetPath подменяет путь к датасету на время теста.
func setDatasetPath(t *testing.T, path string) {
	t.Helper()
	old := datasetPath
	datasetPath = path
	t.Cleanup(func() { datasetPath = old })
}

// writeBrokenXML создаёт во временной папке файл с невалидным XML.
func writeBrokenXML(t *testing.T) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "broken.xml")
	if err := os.WriteFile(p, []byte("<root><row>"), 0644); err != nil {
		t.Fatal(err)
	}
	return p
}

func userIDs(users []User) []int {
	ids := make([]int, 0, len(users))
	for _, u := range users {
		ids = append(ids, u.ID)
	}
	return ids
}

// ---------- loadUsers ----------

func TestLoadUsersTable(t *testing.T) {
	brokenPath := writeBrokenXML(t)

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
	if users[0] != boydWolf {
		t.Errorf("first user:\nwant %+v\n got %+v", boydWolf, users[0])
	}
}

// ---------- FindUsers: поиск, сортировка, пагинация ----------

func TestFindUsersSearch(t *testing.T) {
	ts := newSearchServer(t)
	sc := &SearchClient{AccessToken: validToken, URL: ts.URL}

	cases := []struct {
		name         string
		req          SearchRequest
		wantIDs      []int
		wantNextPage bool
	}{
		{
			name:         "empty order field sorts by name asc",
			req:          SearchRequest{Limit: 5, OrderBy: OrderByAsc},
			wantIDs:      []int{15, 16, 19, 22, 5},
			wantNextPage: true,
		},
		{
			name:         "name desc",
			req:          SearchRequest{Limit: 5, OrderField: "Name", OrderBy: OrderByDesc},
			wantIDs:      []int{13, 33, 18, 26, 9},
			wantNextPage: true,
		},
		{
			name:         "id asc",
			req:          SearchRequest{Limit: 3, OrderField: "Id", OrderBy: OrderByAsc},
			wantIDs:      []int{0, 1, 2},
			wantNextPage: true,
		},
		{
			name:         "id desc",
			req:          SearchRequest{Limit: 3, OrderField: "Id", OrderBy: OrderByDesc},
			wantIDs:      []int{34, 33, 32},
			wantNextPage: true,
		},
		{
			// у 1, 15 и 23 одинаковый возраст 21 — стабильная сортировка сохраняет порядок из файла
			name:         "age asc keeps original order for equal ages",
			req:          SearchRequest{Limit: 5, OrderField: "Age", OrderBy: OrderByAsc},
			wantIDs:      []int{1, 15, 23, 0, 14},
			wantNextPage: true,
		},
		{
			name:         "age desc, lowercase field name",
			req:          SearchRequest{Limit: 5, OrderField: "age", OrderBy: OrderByDesc},
			wantIDs:      []int{13, 32, 6, 26, 31},
			wantNextPage: true,
		},
		{
			name:         "order as is keeps file order",
			req:          SearchRequest{Limit: 4, OrderField: "Age", OrderBy: OrderByAsIs},
			wantIDs:      []int{0, 1, 2, 3},
			wantNextPage: true,
		},
		{
			name:         "example from README: oldest with 'on'",
			req:          SearchRequest{Limit: 1, Query: "on", OrderField: "Age", OrderBy: OrderByDesc},
			wantIDs:      []int{13},
			wantNextPage: true,
		},
		{
			name:    "query matches name",
			req:     SearchRequest{Limit: 10, Query: "Dillard", OrderField: "Id", OrderBy: OrderByAsc},
			wantIDs: []int{3, 17},
		},
		{
			name:    "query matches nothing",
			req:     SearchRequest{Limit: 10, Query: "zzz"},
			wantIDs: []int{},
		},
		{
			name:         "offset",
			req:          SearchRequest{Limit: 3, Offset: 3, OrderField: "Id", OrderBy: OrderByAsc},
			wantIDs:      []int{3, 4, 5},
			wantNextPage: true,
		},
		{
			name:    "last page",
			req:     SearchRequest{Limit: 10, Offset: 30, OrderField: "Id", OrderBy: OrderByAsc},
			wantIDs: []int{30, 31, 32, 33, 34},
		},
		{
			name:    "offset beyond data",
			req:     SearchRequest{Limit: 10, Offset: 100},
			wantIDs: []int{},
		},
		{
			name: "limit above 25 is cut to 25",
			req:  SearchRequest{Limit: 100, OrderField: "Id", OrderBy: OrderByAsc},
			wantIDs: []int{
				0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12,
				13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 24,
			},
			wantNextPage: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resp, err := sc.FindUsers(tc.req)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got := userIDs(resp.Users); !reflect.DeepEqual(got, tc.wantIDs) {
				t.Errorf("ids: want %v, got %v", tc.wantIDs, got)
			}
			if resp.NextPage != tc.wantNextPage {
				t.Errorf("NextPage: want %v, got %v", tc.wantNextPage, resp.NextPage)
			}
		})
	}
}

// Проверяем, что запись приходит целиком, со всеми полями.
func TestFindUsersFullRecord(t *testing.T) {
	ts := newSearchServer(t)
	sc := &SearchClient{AccessToken: validToken, URL: ts.URL}

	resp, err := sc.FindUsers(SearchRequest{Limit: 5, Query: "Boyd"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := []User{boydWolf}
	if !reflect.DeepEqual(resp.Users, want) {
		t.Errorf("want %+v, got %+v", want, resp.Users)
	}
}

// ---------- FindUsers: ошибки ----------

func TestFindUsersErrors(t *testing.T) {
	ts := newSearchServer(t)

	cases := []struct {
		name    string
		token   string
		req     SearchRequest
		wantErr string
	}{
		{
			name:    "negative limit",
			token:   validToken,
			req:     SearchRequest{Limit: -1},
			wantErr: "limit must be > 0",
		},
		{
			name:    "negative offset",
			token:   validToken,
			req:     SearchRequest{Limit: 1, Offset: -1},
			wantErr: "offset must be > 0",
		},
		{
			name:    "bad token",
			token:   "bad",
			req:     SearchRequest{Limit: 1},
			wantErr: "bad AccessToken",
		},
		{
			name:    "bad order field",
			token:   validToken,
			req:     SearchRequest{Limit: 1, OrderField: "Gender"},
			wantErr: "OrderFeld Gender invalid",
		},
		{
			name:    "bad order by",
			token:   validToken,
			req:     SearchRequest{Limit: 1, OrderBy: 5},
			wantErr: "unknown bad request error: " + errBadOrderBy.Error(),
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			sc := &SearchClient{AccessToken: tc.token, URL: ts.URL}
			resp, err := sc.FindUsers(tc.req)
			if err == nil {
				t.Fatalf("expected error, got nil (resp %+v)", resp)
			}
			if err.Error() != tc.wantErr {
				t.Errorf("want err %q, got %q", tc.wantErr, err.Error())
			}
		})
	}
}

func TestFindUsersServerFatal(t *testing.T) {
	cases := []struct {
		name string
		path string
	}{
		{name: "no dataset file", path: "./nope.xml"},
		{name: "broken dataset file", path: writeBrokenXML(t)},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			setDatasetPath(t, tc.path)
			ts := newSearchServer(t)
			sc := &SearchClient{AccessToken: validToken, URL: ts.URL}

			_, err := sc.FindUsers(SearchRequest{Limit: 1})
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if err.Error() != "SearchServer fatal error" {
				t.Errorf("want err %q, got %q", "SearchServer fatal error", err.Error())
			}
		})
	}
}

// Ветки клиента, которые наш правильный сервер не может вызвать сам:
// поднимаем специально «сломанные» обработчики.
func TestFindUsersBrokenServer(t *testing.T) {
	cases := []struct {
		name          string
		handler       http.HandlerFunc
		wantErrPrefix string
	}{
		{
			name: "timeout",
			handler: func(w http.ResponseWriter, r *http.Request) {
				// ждём, пока клиент не отвалится по таймауту
				<-r.Context().Done()
			},
			wantErrPrefix: "timeout for ",
		},
		{
			name: "bad request with broken json",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusBadRequest)
				w.Write([]byte("{not json")) //nolint:errcheck
			},
			wantErrPrefix: "cant unpack error json",
		},
		{
			name: "ok with broken json",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.Write([]byte("{not json")) //nolint:errcheck
			},
			wantErrPrefix: "cant unpack result json",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ts := httptest.NewServer(tc.handler)
			defer ts.Close()

			sc := &SearchClient{AccessToken: validToken, URL: ts.URL}
			_, err := sc.FindUsers(SearchRequest{Limit: 1})
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !strings.HasPrefix(err.Error(), tc.wantErrPrefix) {
				t.Errorf("want err starting with %q, got %q", tc.wantErrPrefix, err.Error())
			}
		})
	}
}

func TestFindUsersServerUnavailable(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(SearchServer))
	url := ts.URL
	ts.Close() // сервер выключен — соединиться не получится

	sc := &SearchClient{AccessToken: validToken, URL: url}
	_, err := sc.FindUsers(SearchRequest{Limit: 1})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.HasPrefix(err.Error(), "unknown error ") {
		t.Errorf("want err starting with %q, got %q", "unknown error ", err.Error())
	}
}

// ---------- SearchServer напрямую ----------

// Клиент всегда шлёт корректные числа, поэтому кривые параметры
// проверяем, вызывая SearchServer напрямую через httptest.NewRecorder.
func TestSearchServerBadParams(t *testing.T) {
	cases := []struct {
		name    string
		query   string
		wantErr error
	}{
		{name: "limit not a number", query: "limit=abc&offset=0&order_by=0", wantErr: errBadLimit},
		{name: "limit missing", query: "offset=0&order_by=0", wantErr: errBadLimit},
		{name: "negative limit", query: "limit=-1&offset=0&order_by=0", wantErr: errBadLimit},
		{name: "offset not a number", query: "limit=1&offset=abc&order_by=0", wantErr: errBadOffset},
		{name: "negative offset", query: "limit=1&offset=-5&order_by=0", wantErr: errBadOffset},
		{name: "order_by not a number", query: "limit=1&offset=0&order_by=abc", wantErr: errBadOrderBy},
		{name: "order_by out of range", query: "limit=1&offset=0&order_by=2", wantErr: errBadOrderBy},
		{name: "bad order_field", query: "limit=1&offset=0&order_by=0&order_field=About", wantErr: errBadOrderField},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/?"+tc.query, nil)
			req.Header.Set("AccessToken", validToken)
			rec := httptest.NewRecorder()

			SearchServer(rec, req)

			if rec.Code != http.StatusBadRequest {
				t.Fatalf("want status %d, got %d", http.StatusBadRequest, rec.Code)
			}
			var errResp SearchErrorResponse
			if err := json.Unmarshal(rec.Body.Bytes(), &errResp); err != nil {
				t.Fatalf("cant unpack error body %q: %v", rec.Body.String(), err)
			}
			if errResp.Error != tc.wantErr.Error() {
				t.Errorf("want error %q, got %q", tc.wantErr.Error(), errResp.Error)
			}
		})
	}
}

func TestSearchServerResponseHeaders(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/?limit=2&offset=0&order_by=1&order_field=Id", nil)
	req.Header.Set("AccessToken", validToken)
	rec := httptest.NewRecorder()

	SearchServer(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("want status 200, got %d", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("want Content-Type application/json, got %q", ct)
	}
	var users []User
	if err := json.Unmarshal(rec.Body.Bytes(), &users); err != nil {
		t.Fatalf("cant unpack body: %v", err)
	}
	if got := userIDs(users); !reflect.DeepEqual(got, []int{0, 1}) {
		t.Errorf("want ids [0 1], got %v", got)
	}
}

// ---------- writeJSON: ошибки сериализации и записи ----------

func TestWriteJSONMarshalError(t *testing.T) {
	old := jsonMarshal
	jsonMarshal = func(any) ([]byte, error) { return nil, errors.New("marshal failed") }
	t.Cleanup(func() { jsonMarshal = old })

	rec := httptest.NewRecorder()
	writeJSON(rec, http.StatusOK, []User{})

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("want status %d, got %d", http.StatusInternalServerError, rec.Code)
	}
}

// failingWriter — ResponseWriter, у которого запись тела всегда падает.
type failingWriter struct {
	*httptest.ResponseRecorder
}

func (failingWriter) Write([]byte) (int, error) {
	return 0, errors.New("write failed")
}

func TestWriteJSONWriteError(t *testing.T) {
	w := failingWriter{httptest.NewRecorder()}
	writeJSON(w, http.StatusOK, []User{})

	// статус успел уйти до ошибки записи тела, тело пустое
	if w.Code != http.StatusOK {
		t.Errorf("want status %d, got %d", http.StatusOK, w.Code)
	}
	if w.Body.Len() != 0 {
		t.Errorf("want empty body, got %q", w.Body.String())
	}
}
