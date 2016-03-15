package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"

	"github.com/gorilla/websocket"
)

// var ES_HOST = "elasticsearch-executor.service.consul"
var ES_HOST = "spaceoddity_database_1"
var ES_PORT = "9200"
var TMP_DATA_DIR = "data/tmp/"
var RH_DATA_DIR = "data/rh/"

var DEV bool

var Verbose bool

var Sockets []*websocket.Conn

func main() {

	var port string
	// Define flags
	flag.BoolVar(&Verbose, "verbose", true, "Verbose output.")
	flag.BoolVar(&DEV, "dev", false, "Run in development, using local CSV file")
	flag.StringVar(&ES_HOST, "es_host", ES_HOST, "Host to connect to")
	flag.StringVar(&ES_PORT, "es_port", ES_PORT, "Port to connect to")
	flag.StringVar(&port, "port", "8081", "Port on which to listen.")
	flag.Parse()

	if Verbose {
		if DEV {
			fmt.Printf("Running with local data %s\n", CSV_FILE)

		} else {
			fmt.Printf("Running on port %s.\n", port)
			fmt.Printf("Connecting to ES on %s port %s\n", ES_HOST, ES_PORT)
		}
	}

	http.HandleFunc("/", defaultHandler)
	http.HandleFunc("/socket", webSocketHandler)
	http.ListenAndServe(":"+port, nil)
}

var (
	upgrader = websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		CheckOrigin:     func(r *http.Request) bool { return true },
	}
)

func webSocketHandler(w http.ResponseWriter, r *http.Request) {
	if Verbose {
		fmt.Printf("Request received: %s\n", r.URL.String())
	}

	if r.Method != "GET" {
		http.Error(w, "Method not allowed", 405)
		return
	}

	ws, err := upgrader.Upgrade(w, r, nil)

	if err != nil {
		fmt.Println("Upgrading to websockets")
		panic(err)
		http.Error(w, "Error Upgrading to websockets", 400)
		return
	}

	Sockets = append(Sockets, ws)
	if Verbose {
		fmt.Printf("Adding web socket. %d sockets open.\n", len(Sockets))
	}

	dataStarted := false
	var status chan<- WebSocketMessage

	for {
		mt, data, err := ws.ReadMessage()
		if err != nil {
			if err == io.EOF {
				fmt.Println("Websocket closed.")
			} else {
				fmt.Println("Error reading websocket message.")
			}
			break
		}

		switch mt {
		case websocket.TextMessage:
			// TODO Validate Message
			if Verbose {
				fmt.Printf("Received %s \n", data)
			}
			// Check if Data has started. If no, start, if yes update
			if !dataStarted {
				status = periodicDataES(ws)
				dataStarted = true
			} 
			var m WebSocketMessage
			err := json.Unmarshal(data, &m)
			check(err)
			status <- m

		case websocket.PingMessage:
			if Verbose {
				fmt.Printf("Received ping: %v\n", data)
			}
			ws.WriteMessage(websocket.PongMessage, []byte{})
		case websocket.CloseMessage:
			if Verbose {
				fmt.Printf("Received close message: %v\n", data)
			}
			err = ws.Close()
			check(err)
		default:
			fmt.Printf("Unknown Message: %v\n", data)
		}
	}

	ws.WriteMessage(websocket.CloseMessage, []byte{})
	err = ws.Close()
	if err != nil {
		fmt.Printf("Error closing socket, %v\n", err)
	}
	// quit <-
	// TODO kill goroutine
}

func initialState() WebSocketMessage {
	// return WebSocketMessage{DataType: "tmp", Altitude: 110, DateTime: "19_12", MinLat: 46.5, MaxLat: 47, MinLon: 7.5, MaxLon: 8.5}
	dataTypes := []string{"rh"}
	return WebSocketMessage{DataType: dataTypes, Altitude: 1700, DateTime: "19_12", MinLat: 0.0, MaxLat: 0.0, MinLon: 0.0, MaxLon: 0.0}
}

// Loop on the files we have from 12:00 on 19th to 09:00 on 20th
func incrementHour(day, hour int) (int, int) {
	if hour == 23 {
		day = 20
		hour = 0
	} else if hour == 9 && day == 20 {
		hour = 12
		day = 19
	} else {
		hour = hour + 1
	}
	return day, hour
}

func defaultHandler(w http.ResponseWriter, r *http.Request) {

	if Verbose {
		fmt.Printf("Request received: %s\n", r.URL.String())
	}

	// TODO - For Basic Auth
	// var USER_NAME = "fakeUser"
	// var PASSWORD = "fakePassword"
	// u, p, ok := r.BasicAuth()
	// if !ok || u != USER_NAME || p != PASSWORD {
	// http.Error(w, "Authorization Failed", http.StatusUnauthorized)
	// return
	// if Verbose {
	// fmt.Println("Basic Auth Failed")
	// }
	// }

	values := r.URL.Query()
	w.Header().Set("Access-Control-Allow-Origin", "*")

	var data []byte
	var err error
	if DEV {
		data, err = json.Marshal(loadFromCsvNew(CSV_FILE, "tmp"))
	} else {
		data, err = json.Marshal(loadFromES(values.Get("index"), ""))
	}

	check(err)
	w.Write(data)
}

func check(err error) {
	if err != nil {
		panic(err)
	}
}

type WeatherData struct {
	Lat   float32 `json:"lat"`
	Lon   float32 `json:"lon"`
	Value float32 `json:"value"`
	Type  string  `json:"type"`
}

type WebSocketMessage struct {
	DataType []string `json:"dataType"`
	Altitude int      `json:"altitude"`
	DateTime string   `json:"dateTime"`
	MinLat   float32  `json:"minLat"`
	MaxLat   float32  `json:"maxLat"`
	MinLon   float32  `json:"minLon"`
	MaxLon   float32  `json:"maxLon"`
	Stop     bool     `json:"stop"`
}