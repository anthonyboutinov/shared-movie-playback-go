package main

import (
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"

	"github.com/gorilla/websocket"
)

var clients = make(map[*websocket.Conn]bool) // Keep a map of all connected clients

func main() {
	http.HandleFunc("/", indexHandler)
	http.HandleFunc("/ws", handleWebsocket)

	http.Handle("/public/", http.StripPrefix("/public/", http.FileServer(http.Dir("./public"))))

	fmt.Println("Starting server at http://localhost:8080")
	err := http.ListenAndServe(":8080", nil)
	if err != nil {
		fmt.Println("Error starting server:", err)
	}
}

func indexHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html")

	tmpl, err := template.ParseFiles("templates/index.html")
	if err != nil {
		fmt.Println(err)
		return
	}

	err = tmpl.Execute(w, nil)
	if err != nil {
		fmt.Println(err)
		return
	}
}

/*
The clients map is used to keep track of all connected clients.
`handleWebsocket` handles incoming websocket connections and adds the new client to the clients map.
The function also listens for incoming messages from the client and broadcasts them to all clients.
*/
func handleWebsocket(w http.ResponseWriter, r *http.Request) {
	// Upgrade the HTTP connection to a websocket connection
	conn, err := websocket.Upgrade(w, r, nil, 1024, 1024)
	if err != nil {
		http.Error(w, "Failed to upgrade to websocket", http.StatusBadRequest)
		return
	}

	// Add the new client to the clients map
	clients[conn] = true

	// Listen for incoming messages from this client
	go func() {
		for {
			_, msg, err := conn.ReadMessage() // Waits for a new message
			if err != nil {
				if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
					fmt.Println("error:", err)
				}
				// Delete problematic client from the list of connected clients
				delete(clients, conn)
				break
			}
			handleMessage(conn, msg)
		}
	}()
}

type Message struct {
	Type string  `json:"type"`
	Time float64 `json:"time"`
}

// func handleMessage(client *Client, message []byte) {
func handleMessage(conn *websocket.Conn, message []byte) {

	defer func() {
		if r := recover(); r != nil {
			conn.Close()
			delete(clients, conn)
			log.Println("Recovered as not nil, removing client")
		}
	}()

	var m Message
	err := json.Unmarshal(message, &m)
	if err != nil {
		log.Println("error:", err)
		return
	}

	if m.Type == "sync-time" || m.Type == "pause" || m.Type == "play" {
		// Broadcast the sync-time/pause/play message to all clients
		for c := range clients {
			if c != conn { // Write to all clients except for the sender of this message
				err := c.WriteJSON(m)
				if err != nil {
					conn.Close()
					delete(clients, conn)
					log.Println("error:", err)
					return
				}
			}
		}
	}
}
