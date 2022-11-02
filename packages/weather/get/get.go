package main

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"time"
)

type Input struct {
	Lat string `json:"lat"`
	Lon string `json:"lon"`
}

type NominatimRes struct {
	PlaceID     int                    `json:"place_id"`
	Licence     string                 `json:"licence"`
	OsmType     string                 `json:"osm_type"`
	OsmID       int                    `json:"osm_id"`
	Lat         string                 `json:"lat"`
	Lon         string                 `json:"lon"`
	DisplayName string                 `json:"display_name"`
	Address     map[string]interface{} `json:"address"`
	Boundingbox []string               `json:"boundingbox"`
}

type AutoGenerated struct {
	WeatherForecasts []struct {
		Location struct {
			Lat float64 `json:"lat"`
			Lon float64 `json:"lon"`
		} `json:"location"`
		Forecasts []struct {
			Time time.Time `json:"time"`
			Data struct {
				Cloudhigh int     `json:"cloudhigh"`
				Cloudlow  int     `json:"cloudlow"`
				Cloudmed  int     `json:"cloudmed"`
				Cond      int     `json:"cond"`
				Rain      int     `json:"rain"`
				Rh        float64 `json:"rh"`
				Slp       float64 `json:"slp"`
				Tc        float64 `json:"tc"`
				Wd10M     float64 `json:"wd10m"`
				Ws10M     float64 `json:"ws10m"`
			} `json:"data"`
		} `json:"forecasts"`
	} `json:"WeatherForecasts"`
}

type Response struct {
	StatusCode int               `json:"statusCode,omitempty"`
	Headers    map[string]string `json:"headers,omitempty"`
	Body       *PackedResponse   `json:"body,omitempty"`
}

type PackedResponse struct {
	Data PrettyResponse `json:"data"`
}

type Condition int

const (
	NO Condition = iota
	CLEAR
	PARTLY_CLOUDY
	CLOUDY
	OVERCAST
	LIGHT_RAIN
	MODERATE_RAIN
	HEAVY_RAIN
	THUNDERSTORM
	VERY_COLD
	COLD
	COOL
	VERY_HOT
)

type PrettyResponse struct {
	Temperature      float64   `json:"temperature"`
	RelativeHumidity float64   `json:"relative_humidity"`
	RainPercentage   int       `json:"rain_percentage"`
	Condition        Condition `json:"condition"`
	Time             time.Time `json:"timestamp"`
	WindSpeed        float64   `json:"wind_speed"'`
	WindDirection    float64   `json:"wind_direction"`
	Location         struct {
		Lat         float64                `json:"lat"`
		Lon         float64                `json:"lon"`
		DisplayName string                 `json:"display_name"`
		Address     map[string]interface{} `json:"address"`
	}
}

func Main(ipt Input) (*Response, error) {
	var response Response
	/**
	Load Environment
	Disabled for FaaS
	*/
	//error := godotenv.Load(".env")
	//if error != nil {
	//	response.Body = error.Error()
	//	response.StatusCode = http.StatusInternalServerError
	//	panic(error)
	//}

	/**
	Initialize HTTP Client
	*/
	client := &http.Client{}

	/**
	Begin to URL Parsing and Add Query
	*/
	url, err := url.Parse(os.Getenv("TMD_API_ENDPOINT") + "/forecast/location/hourly/at")
	q := url.Query()
	q.Add("lat", ipt.Lat)
	q.Add("lon", ipt.Lon)
	q.Add("fields", "tc,rh,slp,rain,ws10m,wd10m,cloudlow,cloudmed,cloudhigh,cond")
	url.RawQuery = q.Encode()

	if err != nil {
		response.Body = nil
		response.StatusCode = http.StatusInternalServerError
	}

	/**
	Begin to Nominatim OpenStreetMap API
	*/
	nominatim, err2 := url.Parse(os.Getenv("NOMINATIM_API_ENDPOINT") + "/reverse")
	query := url.Query()
	query.Add("lat", ipt.Lat)
	query.Add("lon", ipt.Lon)
	query.Add("format", "json")
	nominatim.RawQuery = query.Encode()

	if err2 != nil {
		response.Body = nil
		response.StatusCode = http.StatusInternalServerError
	}
	nominatimReq, err3 := http.NewRequest("GET", nominatim.String(), nil)
	nominatimRes, err3 := client.Do(nominatimReq)
	if err3 != nil {
		response.Body = nil
		response.StatusCode = http.StatusInternalServerError
	}
	nominatimFinal, err := ioutil.ReadAll(nominatimRes.Body)
	if err != nil {
		response.Body = nil
		response.StatusCode = http.StatusInternalServerError
	}
	var nominatimF NominatimRes
	if err := json.Unmarshal(nominatimFinal, &nominatimF); err != nil {
		response.Body = nil
		response.StatusCode = http.StatusInternalServerError
	}

	/**
	Begin HTTP Request
	*/
	req, err := http.NewRequest("GET", url.String(), nil)
	req.Header.Add("Authorization", "Bearer "+os.Getenv("TMD_API_KEY"))
	resp, err := client.Do(req)
	if err != nil {
		response.Body = nil
		response.StatusCode = http.StatusInternalServerError
	}

	/**
	Parsing Body by IOUtil
	*/
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		response.Body = nil
		response.StatusCode = http.StatusInternalServerError
	}

	/**
	Unmarshal Result to JSON and Add Struct to According to Result Body
	*/
	var result AutoGenerated
	if err := json.Unmarshal(body, &result); err != nil { // Parse []byte to go struct pointer
		response.Body = nil
		response.StatusCode = http.StatusInternalServerError
	}

	var prettyResponse PrettyResponse
	prettyResponse.Location.Lat, _ = strconv.ParseFloat(ipt.Lat, 64)
	prettyResponse.Location.Lon, _ = strconv.ParseFloat(ipt.Lon, 64)
	prettyResponse.Time = result.WeatherForecasts[0].Forecasts[0].Time
	prettyResponse.Condition = Condition(result.WeatherForecasts[0].Forecasts[0].Data.Cond)
	prettyResponse.Location.Address = nominatimF.Address
	prettyResponse.Location.DisplayName = nominatimF.DisplayName
	prettyResponse.RainPercentage = result.WeatherForecasts[0].Forecasts[0].Data.Rain
	prettyResponse.RelativeHumidity = result.WeatherForecasts[0].Forecasts[0].Data.Rh
	prettyResponse.WindSpeed = result.WeatherForecasts[0].Forecasts[0].Data.Ws10M
	prettyResponse.WindDirection = result.WeatherForecasts[0].Forecasts[0].Data.Wd10M

	rs := &PackedResponse{
		Data: prettyResponse,
	}
	response.Body = rs
	response.StatusCode = http.StatusOK

	return &Response{
		Body:       response.Body,
		StatusCode: response.StatusCode,
	}, nil
}
