# Web

This is the core webserver for YAGPDB, it handles general stuff like authentication.

Currently it only uses 2 HTTP methods: GET for everything that does not change state, and POST for everything else.

The web package is responsible for handling all the core features of the web suite for yagpdb, authentication, adding the bot to servers, and the other basic core functionality.

It also houses a small form validation toolkit through struct tags (this is all kinda messy but one day i hope to improve everything and make it a lot cleaner and easy to work with).
