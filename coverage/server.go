package main

import (
	"encoding/json"
	"encoding/xml"
	"log"
	"net/http"
	"os"
)

var datasetPath = "./dataset.xml"
const validToken = "secret-token"

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

func SearchServer(w http.ResponseWriter, r *http.Request) {
	accessToken := r.Header.Get("AccessToken")
	if accessToken != validToken {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	users, err := loadUsers(datasetPath)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	data, err := json.Marshal(users)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_, err = w.Write(data)
	if err != nil {
		log.Printf("error: %v", err)
	}
}
