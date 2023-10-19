package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
)

func main() {
	http.ListenAndServe(":5050", http.HandlerFunc(
		func(w http.ResponseWriter, req *http.Request) {
			body, err := ioutil.ReadAll(req.Body)
			if err != nil {
				log.Printf("error reading body: %v", err)
				http.Error(w, fmt.Sprintf("error reading body: %v", err), http.StatusInternalServerError)
				return
			}

			if _, err := w.Write(body); err != nil {
				log.Printf("error writing body: %v", err)
			}
		},
	))
}
