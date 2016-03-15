/*

  This is some socket code hacked to send a JSON message to the drone-data
  websocket, after pushing send on the web interface.
  It should be turned into a proper junit style test, but this would be
  easier with some changes to drone-data.
*/
package main

import (
	"flag"
	"log"
	"net/url"
	"os"
	"os/signal"
	"time"

	"github.com/gorilla/websocket"
)

var addr = flag.String("addr", "drone-data:8081", "http service address")

func main() {
	flag.Parse()
	log.SetFlags(0)

	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt)

	u := url.URL{Scheme: "ws", Host: *addr, Path: "/socket"}
	log.Printf("connecting to %s", u.String())

	c, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		log.Fatal("dial:", err)
	}
	defer c.Close()

	message := `{"DataType": "tmp", "Altitude": 110, "DateTime": "19_12", "MinLat": 47.53, "MaxLat": 47.56, "MinLon": 8.53, "MaxLon": 8.54}`
	err = c.WriteMessage(websocket.TextMessage, []byte(message))
	if err != nil {
		log.Println("write:", err)
		return
	}
	//Try sending twice
	time.Sleep(1)
	err = c.WriteMessage(websocket.TextMessage, []byte(message))
	if err != nil {
		log.Println("write:", err)
		return
	}

	for i := 1; i <= 10; i++ {
		_, message, err := c.ReadMessage()
		if err != nil {
			log.Println("read:", err)
			return
		}
		log.Printf("recv: %s", message)
		time.Sleep(1)
	}
	//Send close
	err = c.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
}
