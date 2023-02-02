const video = document.getElementById("video");

video.addEventListener("play", onVideoPlay);
video.addEventListener("seeked", onVideoSeeked);
video.addEventListener("stalled", onVideoPause);
video.addEventListener("pause", onVideoPause);

let lastInteracted = 0;
let throttleTimerLength = 500;

// function defer(func) {
//     setTimeout(func, 50);
// }

function sendToSocket(data) {
    const now = Date.now();
    if (now - lastInteracted > throttleTimerLength) {
        lastInteracted = now;
        socket.send(JSON.stringify(data));
        console.log("socket.send fired with", data)

        // if (data.type === "sync-time") {
        //     defer(() => {
        //         video.pause();
        //         console.log("Pausing video");
        //     });
        // }
    }
}

function sendVideoControlsActionWithCurrentTime(actionType) {
    sendToSocket({ type: actionType, time: video.currentTime });
}

async function onVideoPause() {
    sendVideoControlsActionWithCurrentTime("pause");
}

async function onVideoPlay() {
    sendVideoControlsActionWithCurrentTime("play")
}

async function onVideoSeeked() {
    sendVideoControlsActionWithCurrentTime("sync-time");
}


// Autoplay restrictions in modern browsers: request permission from the user to autoplay videos
async function askBrowserToPlayVideo() {
    try {
      await video.play();
    } catch (error) {
      if (error.name === "NotAllowedError") {
        // Request user interaction before playing the video
        alert("Please click on the video to allow shared video playback");
        let l = document.body.addEventListener("click", () => video.play());
      }
    }
  }


// Create the socket object
const socket = new WebSocket("ws://localhost:8080/ws");

// Handle incoming messages from the server
socket.onmessage = (event) => {
    console.log("New message with data:", event.data);
    const message = JSON.parse(event.data);

    // Update the video time to the time sent by the server, and play/pause if needed
    const now = new Date();
    if (now - lastInteracted > throttleTimerLength) {
        lastInteracted = now;

        if (message.time) {
            video.currentTime = message.time;
        }

        if (message.type === "pause") {
            video.pause();
        } else if (message.type === "play") {
            askBrowserToPlayVideo();
        }
    }
};

// Handle the socket connection
socket.onopen = () => {
    console.log("socket connected");
};

// Handle socket close event
socket.onclose = () => {
    console.log("socket closed");
    alert("Socket closed. Reload the page to re-establish connection.")
};

// Handle socket error event
socket.onerror = (event) => {
    console.error("socket error:", event);
};