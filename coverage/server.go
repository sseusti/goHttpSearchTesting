package main

import (
	"encoding/json"
	"encoding/xml"
	"errors"
	"log"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strconv"
	"strings"
)

// datasetPath — путь к файлу с данными. Переменная, а не константа,
// чтобы в тестах можно было подменить файл и получить ошибку.
var datasetPath = "./dataset.xml"

const validToken = "secret-token"

// Названия полей сортировки после нормализации.
const (
	orderFieldID   = "id"
	orderFieldAge  = "age"
	orderFieldName = "name"
)

var (
	errBadLimit      = errors.New("limit invalid")
	errBadOffset     = errors.New("offset invalid")
	errBadOrderBy    = errors.New("OrderBy invalid")
	errBadOrderField = errors.New(ErrorBadOrderField)
)

// jsonMarshal вынесен в переменную, чтобы в тесте можно было
// подменить его и проверить ветку с ошибкой сериализации.
var jsonMarshal = json.Marshal

type xmlUser struct {
	ID        int    `xml:"id"`
	FirstName string `xml:"first_name"`
	LastName  string `xml:"last_name"`
	Age       int    `xml:"age"`
	About     string `xml:"about"`
	Gender    string `xml:"gender"`
}

type xmlUsers struct {
	Users []xmlUser `xml:"row"`
}

func loadUsers(path string) ([]User, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var usrs xmlUsers

	err = xml.NewDecoder(file).Decode(&usrs)
	if err != nil {
		return nil, err
	}

	res := make([]User, 0, len(usrs.Users))
	for _, usr := range usrs.Users {
		res = append(res, User{
			ID:     usr.ID,
			Name:   usr.FirstName + " " + usr.LastName,
			Age:    usr.Age,
			About:  usr.About,
			Gender: usr.Gender,
		})
	}

	return res, nil
}

type searchParams struct {
	limit      int
	offset     int
	query      string
	orderField string
	orderBy    int
}

// parseNonNegative разбирает неотрицательное целое число.
// При любой проблеме возвращает переданную ошибку errBad.
func parseNonNegative(value string, errBad error) (int, error) {
	n, err := strconv.Atoi(value)
	if err != nil || n < 0 {
		return 0, errBad
	}
	return n, nil
}

func parseParams(q url.Values) (searchParams, error) {
	limit, err := parseNonNegative(q.Get("limit"), errBadLimit)
	if err != nil {
		return searchParams{}, err
	}

	offset, err := parseNonNegative(q.Get("offset"), errBadOffset)
	if err != nil {
		return searchParams{}, err
	}

	// Принимаем поле в любом регистре: и "Age", и "age".
	orderField := strings.ToLower(q.Get("order_field"))
	switch orderField {
	case "":
		orderField = orderFieldName
	case orderFieldID, orderFieldAge, orderFieldName:
	default:
		return searchParams{}, errBadOrderField
	}

	orderBy, err := strconv.Atoi(q.Get("order_by"))
	if err != nil {
		return searchParams{}, errBadOrderBy
	}
	switch orderBy {
	case OrderByAsc, OrderByAsIs, OrderByDesc:
	default:
		return searchParams{}, errBadOrderBy
	}

	return searchParams{
		limit:      limit,
		offset:     offset,
		query:      q.Get("query"),
		orderField: orderField,
		orderBy:    orderBy,
	}, nil
}

// filterUsers оставляет юзеров, у которых query встречается в Name или About.
// Пустой query означает «без фильтра».
func filterUsers(users []User, query string) []User {
	if query == "" {
		return users
	}

	res := make([]User, 0, len(users))
	for _, u := range users {
		if strings.Contains(u.Name, query) || strings.Contains(u.About, query) {
			res = append(res, u)
		}
	}
	return res
}

// sortUsers сортирует слайс на месте. Сортировка стабильная,
// чтобы юзеры с одинаковым значением поля шли в исходном порядке.
func sortUsers(users []User, orderField string, orderBy int) {
	if orderBy == OrderByAsIs {
		return
	}

	less := func(a, b User) bool {
		switch orderField {
		case orderFieldID:
			return a.ID < b.ID
		case orderFieldAge:
			return a.Age < b.Age
		default:
			return a.Name < b.Name
		}
	}

	sort.SliceStable(users, func(i, j int) bool {
		if orderBy == OrderByDesc {
			return less(users[j], users[i])
		}
		return less(users[i], users[j])
	})
}

// paginate возвращает не более limit юзеров, начиная с индекса offset.
func paginate(users []User, offset, limit int) []User {
	if offset >= len(users) {
		return []User{}
	}

	end := offset + limit
	if end > len(users) {
		end = len(users)
	}
	return users[offset:end]
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	data, err := jsonMarshal(v)
	if err != nil {
		log.Printf("marshal response: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if _, err := w.Write(data); err != nil {
		log.Printf("write response: %v", err)
	}
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, SearchErrorResponse{Error: msg})
}

func SearchServer(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("AccessToken") != validToken {
		writeError(w, http.StatusUnauthorized, "bad AccessToken")
		return
	}

	params, err := parseParams(r.URL.Query())
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	users, err := loadUsers(datasetPath)
	if err != nil {
		log.Printf("load users: %v", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	users = filterUsers(users, params.query)
	sortUsers(users, params.orderField, params.orderBy)
	users = paginate(users, params.offset, params.limit)

	writeJSON(w, http.StatusOK, users)
}
