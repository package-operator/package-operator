package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"net/http"

	"github.com/gorilla/mux"
)

func main() {
	r := mux.NewRouter()
	r.HandleFunc("/healthz", Health)
	r.HandleFunc("/readyz", Health)
	r.HandleFunc(
		"/api/clusters_mgmt/v1/clusters/{cluster_id}/upgrade_policies/{upgrade_policy_id}/state",
		UpgradePolicyState,
	)

	addr := ":8080"
	log.Printf("listening on %s\n", addr)
	if err := http.ListenAndServe(addr, r); err != nil {
		panic(err)
	}
}

func Health(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
}

func UpgradePolicyState(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPatch {
		w.WriteHeader(http.StatusNotImplemented)
		return
	}

	w.WriteHeader(http.StatusOK)
	vars := mux.Vars(r)
	fmt.Fprintf(w, "Category: %v\n", vars["category"])

	payload, err := ioutil.ReadAll(r.Body)
	if err != nil {
		panic(err)
	}
	defer r.Body.Close()

	log.Printf("%s %s:\n%s\n", r.URL.String(), r.Method, payload)
}
