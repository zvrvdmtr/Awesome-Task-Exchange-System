package main

import (
	"context"
	"io/ioutil"
	"log"
	"net/http"
	"os/signal"
	"syscall"

	"github.com/xeipuuv/gojsonschema"
)

func main() {

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	http.HandleFunc("/validate_task", ValidateTask())
	http.HandleFunc("/validate_client", ValidateClient())

	//run service
	go func() {
		if err := http.ListenAndServe(":8083", nil); err != nil {
			log.Fatalf("listen: %s\n", err)
		}
		stop()
	}()
	log.Println("schema registry service started on :8083")

	<-ctx.Done()
}

func ValidateTask() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		body, err := ioutil.ReadAll(r.Body)
		if err != nil {
			log.Printf("can`t read body: %s", err.Error())
		}
		schemaLoader := gojsonschema.NewReferenceLoader("file:///Users/d.zverev/GolandProjects/Awesome-Task-Exchange-System/schemaregistry/task/1.json")
		bodyLoader := gojsonschema.NewBytesLoader(body)
		result, err := gojsonschema.Validate(schemaLoader, bodyLoader)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		if result.Valid() {
			w.WriteHeader(http.StatusOK)
		} else {
			log.Println(err.Error())
			w.WriteHeader(http.StatusBadRequest)
		}
	}
}

func ValidateClient() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		body, err := ioutil.ReadAll(r.Body)
		if err != nil {
			log.Printf("can`t read body: %s", err.Error())
		}
		schemaLoader := gojsonschema.NewReferenceLoader("file:///Users/d.zverev/GolandProjects/Awesome-Task-Exchange-System/schemaregistry/client/1.json")
		bodyLoader := gojsonschema.NewBytesLoader(body)
		result, err := gojsonschema.Validate(schemaLoader, bodyLoader)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		if result.Valid() {
			w.WriteHeader(http.StatusOK)
		} else {
			log.Println(err.Error())
			w.WriteHeader(http.StatusBadRequest)
		}
	}
}
