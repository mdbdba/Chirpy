package main

import (
	"encoding/json"
	"net/http"
)

func respondWithJSON(w http.ResponseWriter, code int, payload interface{}) {
	w.WriteHeader(code)
	response, err := json.Marshal(payload)
	// fmt.Println(string(dat))
	if err != nil {
		return
	}
	w.Write(response)
}

func respondWithError(w http.ResponseWriter, code int, responseString string) {
	type errorResp struct {
		Error string `json:"error"`
	}
	w.WriteHeader(code)
	errorResponse := errorResp{Error: responseString}
	dat, err := json.Marshal(errorResponse)
	if err != nil {
		return
	}
	_, err = w.Write(dat)
	if err != nil {
		return
	}
	return
}
