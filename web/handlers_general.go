package main

import (
	"golang.org/x/net/context"
	"log"
	"net/http"
)

func index(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	templateData := make(map[string]interface{})
	session := DiscordSessionFromContext(ctx)
	log.Println("session", session == nil)
	if session != nil {
		user, err := session.User("@me")
		if err != nil {
			log.Println("Error fetching user data", err)
		} else {
			log.Println("Sucess!", user.Username)
			templateData["logged_in"] = true
			templateData["user"] = user
		}
	}

	err := templates.ExecuteTemplate(w, "index", templateData)
	if err != nil {
		log.Println("Failed executing templae", err)
	}
}
