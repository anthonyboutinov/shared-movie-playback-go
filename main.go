package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"github.com/gorilla/websocket"
)

var clients = make(map[*websocket.Conn]bool) // Keep a map of all connected clients
var broadcast = make(chan string)            // Channel to broadcast messages to all clients

func main() {
	http.HandleFunc("/", indexHandler)
	http.HandleFunc("/pause", pauseHandler)
	http.HandleFunc("/unpause", unpauseHandler)
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
	fmt.Fprintln(w, `
	<html>
		<head>
			<title>Shared Movie Playback</title>
			<link rel="stylesheet" href="https://cdn.jsdelivr.net/npm/bulma@0.9.4/css/bulma.min.css">
		</head>
		<body>
			<video id="video" src="/public/sample.mkv" controls class="has-ratio is-16by9 is-fullwidth" style="width:80%;">
				Your browser does not support the video tag (.mkv).
			</video>
			<div>
				<button onclick="pauseVideo()" class="button is-rounded p-1">Pause</button>
				<button onclick="unpauseVideo()" class="button is-rounded p-1">Play</button>
				<button onclick="syncTime()" class="button is-rounded p-1">Sync Time</button>
			</div>
			<script>
				const video = document.getElementById("video");

				video.addEventListener("pause", pauseVideo);
				video.addEventListener("play", unpauseVideo);
				video.addEventListener("seeked", syncTime);

				let modifyingVideoState = false;

				async function pauseVideo() {
					console.log("pauseVideo while modifyingVideoState=", modifyingVideoState)
					if (modifyingVideoState) {
						return;
					}
					modifyingVideoState = true;
					await fetch("/pause");
					video.pause();
					modifyingVideoState = false;
				}

				async function unpauseVideo() {
					console.log("unpauseVideo while modifyingVideoState=", modifyingVideoState)
					if (modifyingVideoState) {
						return;
					}
					modifyingVideoState = true;
					await fetch("/unpause");
					video.play();
					modifyingVideoState = false;
				}

				let lastSentTime = 0;

				async function syncTime() {
					const now = Date.now();
					if (now - lastSentTime > 500) {
						lastSentTime = now;
						// Send a message to the server with the current time
						socket.send(JSON.stringify({ type: "sync-time", time: video.currentTime }));
						console.log("socket.send fired with ", { type: "sync-time", time: video.currentTime }, new Date())
					}
				}

				// Create the socket object
				const socket = new WebSocket("ws://localhost:8080/ws");

				// Handle incoming messages from the server
				socket.onmessage = (event) => {
					console.log(event);
					if (event.data === "pause") {
						video.pause();
					} else if (event.data === "unpause") {
						video.play();
					} else {
					const message = JSON.parse(event.data);
						if (message.type === "sync-time") {
							// Update the video time to the time sent by the server
							video.currentTime = message.time;
						}
					}
				};

				// Handle the socket connection
				socket.onopen = (event) => {
					console.log("socket connected");
				};

				// Handle socket close event
				socket.onclose = (event) => {
					console.log("socket closed");
				};

				// Handle socket error event
				socket.onerror = (event) => {
					console.error("socket error:", event);
				};
			</script>
		</body>
	</html>
	`)
}

func pauseHandler(w http.ResponseWriter, r *http.Request) {
	broadcast <- "pause"
	w.Write([]byte("Paused"))
}

func unpauseHandler(w http.ResponseWriter, r *http.Request) {
	broadcast <- "unpause"
	w.Write([]byte("Unpaused"))
}

/*
In this code, the clients map is used to keep track of all connected clients, and the broadcast channel is used to send messages to all clients. The handleWebsocket function handles incoming websocket connections and adds the new client to the clients map. The function also listens for incoming messages from the client and broadcasts them to all clients. The pauseHandler and unpauseHandler functions send messages to the broadcast
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
			_, msg, err := conn.ReadMessage()
			if err != nil {
				if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
					fmt.Println("error:", err)
				}
				delete(clients, conn)
				break
			}
			broadcast <- string(msg)
			handleMessage(conn, msg)
		}
	}()

	// Send outgoing messages to this client
	for msg := range broadcast {
		err := conn.WriteMessage(websocket.TextMessage, []byte(msg))
		if err != nil {
			delete(clients, conn)
			break
		}
	}
}

type Message struct {
	Type string  `json:"type"`
	Time float64 `json:"time"`
}

// type Client struct {
// 	conn *websocket.Conn
// 	send chan []byte
// }

// func handleMessage(client *Client, message []byte) {
func handleMessage(conn *websocket.Conn, message []byte) {

	defer func() {
		if r := recover(); r != nil {
			conn.Close()
			delete(clients, conn)
			log.Println("recovered as not nil, removing client")
		}
	}()

	var m Message
	err := json.Unmarshal(message, &m)
	if err != nil {
		log.Println("error:", err)
		return
	}

	if m.Type == "sync-time" {
		// Broadcast the sync-time message to all clients
		for c := range clients {
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
