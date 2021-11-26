package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"sync"

	"github.com/gorilla/mux"
)

func main() {
	r := mux.NewRouter()
	r.HandleFunc("/healthz", Health)
	r.HandleFunc("/readyz", Health)
	r.Handle(
		"/api/clusters_mgmt/v1/clusters/{cluster_id}/upgrade_policies/{upgrade_policy_id}/state",
		NewUpgradePolicyStateEndpoint(),
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

type UpgradePolicyStateEndpoint struct {
	data    map[UpgradePolicyStateKey]string
	dataMux sync.RWMutex
}

func NewUpgradePolicyStateEndpoint() *UpgradePolicyStateEndpoint {
	return &UpgradePolicyStateEndpoint{
		data: map[UpgradePolicyStateKey]string{},
	}
}

type UpgradePolicyStateKey struct {
	ClusterID, UpgradePolicyID string
}

func (ups *UpgradePolicyStateEndpoint) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPatch:
		ups.dataMux.Lock()
		defer ups.dataMux.Unlock()

		payload, err := ioutil.ReadAll(r.Body)
		if err != nil {
			log.Printf("reading request body: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintln(w, `{}`)
			return
		}

		vars := mux.Vars(r)
		ups.data[UpgradePolicyStateKey{
			ClusterID:       vars["cluster_id"],
			UpgradePolicyID: vars["upgrade_policy_id"],
		}] = string(payload)

		w.WriteHeader(http.StatusOK)
		fmt.Fprintln(w, `{}`)
		log.Printf("%s %s:\n%s\n", r.URL.String(), r.Method, payload)

	case http.MethodGet:
		ups.dataMux.RLock()
		defer ups.dataMux.RUnlock()

		vars := mux.Vars(r)
		data, ok := ups.data[UpgradePolicyStateKey{
			ClusterID:       vars["cluster_id"],
			UpgradePolicyID: vars["upgrade_policy_id"],
		}]
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			fmt.Fprintln(w, `{}`)
			return
		}

		w.WriteHeader(http.StatusOK)
		fmt.Fprintln(w, data)
		log.Printf("%s %s:\n", r.URL.String(), r.Method)

	default:
		w.WriteHeader(http.StatusNotImplemented)
		return
	}
}
