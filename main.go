package main

import (
	"io/ioutil"
	"log"
	"net/http"

	"github.com/milescrabill/mirror-server/config"

	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
)

var (
	conf config.Config
)

func WeatherHandler(w http.ResponseWriter, req *http.Request) {
	resp, err := http.Get("https://api.forecast.io/forecast/" + conf.ForecastToken + "/37.8267,-122.423")
	if err != nil {
		log.Fatal(err.Error())
	}
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Fatal(err.Error())
	}
	w.Write(body)
}

func main() {
	conf = config.GetConfig()

	r := mux.NewRouter()
	r.HandleFunc("/weather", WeatherHandler)
	http.ListenAndServe(":8000", handlers.CORS()(r))
}
