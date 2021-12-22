package main

import (
	"context"
	"database/sql"
	_ "encoding/json"
	"fmt"
	"github.com/go-redis/redis/v8"
	"github.com/gorilla/websocket"
	"github.com/joho/godotenv"
	_ "github.com/mattn/go-sqlite3"
	_ "html/template"
	"io"
	"log"
	"net/http"
	_ "net/http/cookiejar"
	"os"
	_ "time"
)

var ctx = context.Background()

var (
	rdb *redis.Client
)

type Message struct {
	Username string `json:"username"`
	Text     string `json:"text"`
}

var clients = make(map[*websocket.Conn]bool) // connected clients

var broadcast = make(chan Message) // broadcast channel

// Configure the upgrader
var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}
var db, _ = sql.Open("sqlite3", "./database.db")

func handleConnections(w http.ResponseWriter, r *http.Request) {
	// Upgrade initial GET request to a websocket
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Fatal(err)
	}
	// Make sure we close the connection when the function returns
	defer ws.Close()

	// Register our new client
	clients[ws] = true

	// send previous messages
	var rows, _ = db.Query("select * from messages;")
	var mess Message
	for rows.Next() {
		_ = rows.Scan(&mess.Username, &mess.Text)
		err := ws.WriteJSON(mess)
		if err != nil {
			log.Printf("error: %v", err)
		}
	}
	rows.Close()

	for {
		var msg Message
		// Read in a new message as JSON and map it to a Message object
		err := ws.ReadJSON(&msg)
		if err != nil {
			log.Printf("error: %v", err)
			delete(clients, ws)
			break
		}
		// Send the newly received message to the broadcast channel
		broadcast <- msg
	}
}

func handleMessages() {
	for {
		// Grab the next message from the broadcast channel
		msg := <-broadcast
		// Send it out to every client that is currently connected
		stmt, _ := db.Prepare("INSERT INTO Messages(username,text) VALUES(?,?)")
		stmt.Exec(msg.Username, msg.Text)
		messageClients(msg)
	}
}

func messageClients(msg Message) {
	for client := range clients {
		err := client.WriteJSON(msg)
		if err != nil {
			log.Printf("error: %v", err)
			client.Close()
			delete(clients, client)
		}
	}
}

// If a message is sent while a client is closing, ignore the error
func unsafeError(err error) bool {
	return !websocket.IsCloseError(err, websocket.CloseGoingAway) && err != io.EOF
}
func LoginHandler(w http.ResponseWriter, r *http.Request) {
	// fmt.Fprintf(w, "POST request successful\n")
	username := r.FormValue("username")
	password := r.FormValue("password")
	// fmt.Fprintf(w, "username:%s\n", username)
	// fmt.Fprintf(w, "password:%s\n", password)
	var rows, _ = db.Query(fmt.Sprintf("SELECT password FROM user WHERE username='%s'", username))
	var pass string
	for rows.Next() {
		_ = rows.Scan(&pass)
		fmt.Println(pass)
	}
	rows.Close()
	if password == pass {
		cookie := &http.Cookie{
			Name:  "username",
			Value: username,
		}
		http.SetCookie(w, cookie)
		http.Redirect(w, r, "chat.html", 301)
	} else {
		http.Redirect(w, r, "logare.html", 301)
	}
}
func RegisterHandler(w http.ResponseWriter, r *http.Request) {
	username := r.FormValue("username")
	password := r.FormValue("password")
	var rows, _ = db.Query(fmt.Sprintf("SELECT username,password FROM user WHERE username='%s'", username))
	count := 0
	for rows.Next() {
		count = count + 1
	}
	rows.Close()
	if count == 0 {
		stmt, _ := db.Prepare("INSERT INTO USER(username,password) VALUES(?,?)")
		stmt.Exec(username, password)
	}

	cookie := &http.Cookie{
		Name:  "username",
		Value: username,
	}
	http.SetCookie(w, cookie)
	http.Redirect(w, r, "chat.html", 301)
}

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}
	port := os.Getenv("PORT")
	fileServer := http.FileServer(http.Dir("./"))
	if err != nil {
		panic(err)
	}
	http.Handle("/", fileServer)
	http.HandleFunc("/logare", LoginHandler)
	http.HandleFunc("/inregistrare", RegisterHandler)
	http.HandleFunc("/websocket", handleConnections)
	go handleMessages()
	fmt.Printf("Starting server at port 8080\n")
	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Fatal(err)
	}

	// stmt, _ := db.Prepare("INSERT INTO USER(username,password) VALUES(?,?)")

	// stmt.Exec("user", "parola")

}
