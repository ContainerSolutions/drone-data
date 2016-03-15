package main

import (
	"encoding/csv"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/websocket"
)

var CSV_FILE = "/data/drone.csv"

var LON_COL = 0
var LAT_COL = 1
var VAL_COL = 2

func periodicDataCSV(ws *websocket.Conn) chan<- WebSocketMessage {
	var day = 19
	var hour = 12

	ticker := time.NewTicker(2 * time.Second)
	status := make(chan WebSocketMessage)
	quit := make(chan struct{})
	go func() {
		state := initialState()
		for {
			select {
			case <-ticker.C:
				// data, err := json.Marshal(loadFromES("*", "{\"range\":{\"date\":{\"gte\":\"now-5s/s\",\"lt\":\"now/s\"}}}"))
				var data []WeatherData
				tmp := false
				rh := false
				for _, d := range state.DataType {
					if d == "tmp" {
						tmp = true
					} else if d == "rh" {
						rh = true
					}
				}
				if tmp {
					f := TMP_DATA_DIR + "tmp_" + strconv.Itoa(state.Altitude) + "m_201512" + strconv.Itoa(day) + "_" + strconv.Itoa(hour) + "Z.XYZ"
					if Verbose {
						fmt.Printf("Sending data from file: %s\n", f)
					}
					data = loadFromCsvNew(f, "tmp")

					// Parse out data by Lat Long
					var latLonData []WeatherData

					// fmt.Printf("MaxLat %f MinLat %f MaxLon %f MinLon %f\n", state.MaxLat, state.MinLat, state.MaxLon, state.MinLon)

					for _, d := range data {
						if d.Lat <= state.MaxLat &&
							d.Lat >= state.MinLat &&
							d.Lon <= state.MaxLon &&
							d.Lon >= state.MinLon {
							latLonData = append(latLonData, d)
						}
					}
					day, hour = incrementHour(day, hour)

					err := ws.WriteJSON(latLonData)
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
					f := RH_DATA_DIR + "rh_" + strconv.Itoa(state.Altitude) + "m_201512" + strconv.Itoa(day) + "_" + strconv.Itoa(hour) + "Z.XYZ"
					if Verbose {
						fmt.Printf("Sending data from file: %s\n", f)
					}
					// rhData := loadFromCsvNew(f, "rh")
					// data = append(data, loadFromCsvNew(f, "rh")...)
					data = loadFromCsvNew(f, "rh")

					// Parse out data by Lat Long
					var latLonData []WeatherData

					// fmt.Printf("MaxLat %f MinLat %f MaxLon %f MinLon %f\n", state.MaxLat, state.MinLat, state.MaxLon, state.MinLon)
					for _, d := range data {
						if d.Lat <= state.MaxLat &&
							d.Lat >= state.MinLat &&
							d.Lon <= state.MaxLon &&
							d.Lon >= state.MinLon {
							latLonData = append(latLonData, d)
						}
					}
					day, hour = incrementHour(day, hour)

					err := ws.WriteJSON(latLonData)
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

			case s := <-status:
				fmt.Println("Received update")
				// Only update fields which were sent
				fmt.Printf("Received: %v\n", s)

								// FOG should only be sent on request
				if len(s.DataType) > 0 && s.DataType[0] == "fog" && !s.Stop {
					pieces := strings.Split(s.DateTime, "_")
					if len(pieces) < 2 {
						fmt.Printf("Missing data time piece for Fog request.\n")
						continue
					}

					f := RH_DATA_DIR + "rh_" + strconv.Itoa(s.Altitude) + "m_201512" + pieces[0] + "_" + pieces[1] + "Z.XYZ"
					if Verbose {
						fmt.Printf("Sending FOG data from file: %s\n", f)
					}

					data := loadFromCsvNew(f, "fog")
					// Parse out data by Lat Long
					var latLonData []WeatherData

					for _, d := range data {
						if d.Lat <= state.MaxLat &&
							d.Lat >= state.MinLat &&
							d.Lon <= state.MaxLon &&
							d.Lon >= state.MinLon {
							latLonData = append(latLonData, d)
						}
					}

					err := ws.WriteJSON(latLonData)
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


func loadFromCsvNew(file string, dataType string) []WeatherData {

	f, err := os.Open(file)
	if err != nil {
		fmt.Printf("Can't open file: %s\n", file, err)
		return nil
	}

	r := csv.NewReader(f)
	r.Comma = ' '
	csvData, err := r.ReadAll()
	check(err)

	var drones []WeatherData
	var drone WeatherData
	for i, data := range csvData {
		if i == 0 {
			// skip header line, TODO test for header?
			continue
		}
		drone = WeatherData{}
		val, err := strconv.ParseFloat(strings.TrimSpace(data[LAT_COL]), 32)
		check(err)
		drone.Lat = float32(val)
		val, err = strconv.ParseFloat(strings.TrimSpace(data[LON_COL]), 32)
		check(err)
		drone.Lon = float32(val)
		val, err = strconv.ParseFloat(strings.TrimSpace(data[VAL_COL]), 32)
		check(err)
		drone.Value = float32(val)
		drone.Type = dataType
		drones = append(drones, drone)
	}

	return drones
}
