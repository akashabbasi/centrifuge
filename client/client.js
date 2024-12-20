import express from "express";
import { Centrifuge } from "centrifuge"; // Official Centrifuge client
import WebSocket from "ws"; // WebSocket polyfill for Node.js

// Set the WebSocket polyfill for Node.js
global.WebSocket = WebSocket;

const jwtToken =
  "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJleHAiOjE3MzA2MzMzOTAsInJvbGUiOiJ1c2VyIiwic3ViIjoiMiJ9.3b16YaxJNGav4V5YnFA8L8Uy1h6-xszrtW1rGg5qhiw"
const centrifuge = new Centrifuge("ws://localhost:8001/connection/websocket", {
  token: jwtToken,
  websocket: WebSocket,
  
});

// Create an Express app and listen on port 3000
const app = express();

// Setup a basic route for health check or testing
app.get("/", (req, res) => {
  res.send("Centrifuge Node.js Client is running!");
});

// Run the client on port 3000
app.listen(3000, () => {
  console.log("Server running on http://localhost:3000");

  centrifuge.on("connecting", function (ctx) {
    console.log("connecting", ctx);
  });

  centrifuge.on("publication", (ctx) => {
    console.log("server side subscription publication: ", ctx)
  })

  // Handle successful connection
  centrifuge.on("connected", (ctx) => {
    console.log("Connected to Centrifugo:", ctx);

    // Subscribe to a channel (e.g., "chat_room_1")
    const subscription = centrifuge.newSubscription("markets:btcusdt");

    // Handle messages from the channel
    subscription.on("publication", (ctx) => {
      console.log("New message from chat_room_1:", ctx);
    });

    // Handle successful subscription
    subscription.on("subscribed", (ctx) => {
      console.log("Subscribed to chat_room_1:", ctx);
    });

    subscription.on("subscribing", (ctx) => {
      console.log("Subscribing to chat_room_1:", ctx);
    });

    // Handle subscription errors
    subscription.on("error", (ctx) => {
      console.error("Subscription error:", ctx);
    });

    // Handle unsubscribing
    subscription.on("unsubscribed", (ctx) => {
      console.log("Unsubscribed from chat_room_1:", ctx);
    });

    subscription.subscribe();
  });

  // Handle connection errors
  centrifuge.on("disconnected", (ctx) => {
    console.error("Disconnected from Centrifugo:", ctx);
  });

  // Handle connection failures
  centrifuge.on("error", (ctx) => {
    console.error("Connection error:", ctx);
  });

  // Connect to Centrifugo
  centrifuge.connect();
});
