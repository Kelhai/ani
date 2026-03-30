package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

type loginUser struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func main() {
	user := loginUser{
		Username: "kellan",
		Password: "mypassword",
	}
	jsonUser, err := json.Marshal(user)
	if err != nil {
		fmt.Println("Error marshalling user")
	}

	req, err := http.NewRequest("POST", "http://localhost:52971/auth/login", bytes.NewBuffer(jsonUser))
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()

	fmt.Println("response Status:", resp.Status)
	fmt.Println("response Headers:", resp.Header)
	body, _ := io.ReadAll(resp.Body)
	fmt.Println("response Body:", string(body))

}
