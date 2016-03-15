package main

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/gorilla/websocket"
	"github.com/mattbaird/elastigo/lib"
)

func periodicDataES(ws *websocket.Conn) chan<- WebSocketMessage {
	var day = 19
	var hour = 12

	ticker := time.NewTicker(1 * time.Second)
	status := make(chan WebSocketMessage)
	quit := make(chan struct{})
	go func() {
		state := initialState()
		for {
			select {
			case <-ticker.C:
				// var data []WeatherData
				tmp := false
				rh := false
				for _, d := range state.DataType {
					if d == "tmp" {
						tmp = true
					} else if d == "rh" {
						rh = true
					}
				}

				index := fmt.Sprintf("drone-prod-12-%d-%d", day, hour)
				//I realise a printf into JSON isn't the safest way to do this
				//but it should work...

				if tmp {
					query := fmt.Sprintf(`
{
  "query" : {
    "filtered" :{
      "filter": {
         "and": [
           { "term" : { "month" : 12 }},
           { "range" : {
              "lat" : {
                "from" : %f,
                "to" : %f
              },
              "lon" : {
                "from" : %f,
                "to" : %f
              }
           }},
           { "term" : {"data_type" : "tmp" }},
           { "term" : { "height" : 1700 }}
        ]
      }
    }
  }
}`, state.MinLat, state.MaxLat, state.MinLon, state.MaxLon)

					data := loadFromES(index, query)

					err := ws.WriteJSON(data)
					if err != nil {
						ws.WriteMessage(websocket.CloseMessage, []byte{})
						err = ws.Close()
						if err != nil {
							fmt.Printf("Error closing socket, %v\n", err)
							ticker.Stop()
							return
						}
					}
				}
				if rh {
					query := fmt.Sprintf(`
{
  "query" : {
    "filtered" :{
      "filter": {
         "and": [
           { "term" : { "month" : 12 }},
           { "range" : {
              "lat" : {
                "from" : %f,
                "to" : %f
              },
              "lon" : {
                "from" : %f,
                "to" : %f
              }
           }},
           { "term" : {"data_type" : "rh" }},
           { "term" : { "height" : 1700 }}
        ]
      }
    }
  }
}`, state.MinLat, state.MaxLat, state.MinLon, state.MaxLon)

					data := loadFromES(index, query)

					err := ws.WriteJSON(data)
					if err != nil {
						ws.WriteMessage(websocket.CloseMessage, []byte{})
						err = ws.Close()
						if err != nil {
							fmt.Printf("Error closing socket, %v\n", err)
							ticker.Stop()
							return
						}
					}
				}
				day, hour = incrementHour(day, hour)
			case s := <-status:
				fmt.Printf("Received Update: %v\n", s)

								// FOG should only be sent on request
				if len(s.DataType) > 0 && s.DataType[0] == "fog" && !s.Stop {
					pieces := strings.Split(s.DateTime, "_")
					if len(pieces) < 2 {
						fmt.Printf("Missing data time piece for Fog request.\n")
						continue
					}
				index := fmt.Sprintf("drone-prod-12-%s-%s", pieces[0], pieces[1])

				query := fmt.Sprintf(`
{
  "query" : {
    "filtered" :{
      "filter": {
         "and": [
           { "term" : { "month" : 12 }},
           { "range" : {
              "lat" : {
                "from" : %f,
                "to" : %f
              },
              "lon" : {
                "from" : %f,
                "to" : %f
              }
           }},
           { "term" : {"data_type" : "rh" }},
           { "term" : { "height" : 1700 }}
        ]
      }
    }
  }
}`, state.MinLat, state.MaxLat, state.MinLon, state.MaxLon)

					data := loadFromES(index, query)

					for i, d := range data {
						d.Type = "fog"
						data[i] = d
					}
					err := ws.WriteJSON(data)
					if err != nil {
						ws.WriteMessage(websocket.CloseMessage, []byte{})
						err = ws.Close()
						if err != nil {
							fmt.Printf("Error closing socket, %v\n", err)
						}
					}
				} else {
					if len(s.DataType) > 0 {
						if s.Stop {
							// Handle case of stopping already stopped....
							fmt.Printf("Stop. Pre: %s\n", state.DataType)
							for i, d := range state.DataType {
								if d == s.DataType[0] {
									state.DataType = append(state.DataType[:i], state.DataType[i+1:]...)
								}
							}
						} else {
							state.DataType = append(state.DataType, s.DataType...)
						}
					}
					if s.Altitude > 0 {
						state.Altitude = s.Altitude
					}
					if len(s.DateTime) > 0 {
						state.DateTime = s.DateTime
					}
					if s.MinLat != 0 {
						state.MinLat = s.MinLat
					}
					if s.MaxLat != 0 {
						state.MaxLat = s.MaxLat
					}
					if s.MinLon != 0 {
						state.MinLon = s.MinLon
					}
					if s.MaxLon != 0 {
						state.MaxLon = s.MaxLon
					}
				}
			case <-quit:
				ticker.Stop()
				return
			}
		}
	}()
	return status
}

func loadFromES(index string, query string) []WeatherData {

	if Verbose {
		fmt.Printf("Fetching data from Elasticsearch. Index: %s Query: %s\n", index, query)
	}

	c := elastigo.NewConn()
	c.Domain = ES_HOST
	c.Port = ES_PORT
	var drones []WeatherData

	out, err := c.Search(index, "", map[string]interface{}{"size": "10000"}, query)
	// check(err)
	if err != nil {
		fmt.Printf("Received error from ES: %v", err)
		return drones
	}

	var d WeatherData
	var es EsData

	for i := 0; i < len(out.Hits.Hits); i++ {
		t := out.Hits.Hits[i]
		// fmt.Println(string(*t.Source))
		err = json.Unmarshal(*t.Source, &es)
		check(err)
		d = WeatherData{Lat: es.Lat, Lon: es.Lon, Value: es.Val, Type: es.DataType}
		// fmt.Printf("Weather Data: %v\n", d)
		drones = append(drones, d)
	}

	if Verbose {
		fmt.Printf("Received %d records.\n", len(drones))
	}

	return drones
}

type EsData struct {
	DataType string  `json:"data_type"`
	Lat      float32 `json:"lat"`
	Lon      float32 `json:"lon"`
	Val      float32 `json:"val"`
}
