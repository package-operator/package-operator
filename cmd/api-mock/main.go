package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"regexp"
	"strings"
	"sync"

	"github.com/gorilla/mux"
)

func main() {
	r := mux.NewRouter()
	r.HandleFunc("/healthz", Health)
	r.HandleFunc("/readyz", Health)
	r.Handle(
		"/api/clusters_mgmt/v1/clusters",
		NewClustersEndpoint(),
	)
	r.Handle(
		"/api/clusters_mgmt/v1/clusters/{cluster_id}/addon_upgrade_policies/{upgrade_policy_id}/state",
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

type ClustersEndpoint struct {
	data    map[ClustersKey]string
	dataMux sync.RWMutex
}

func NewClustersEndpoint() *ClustersEndpoint {
	return &ClustersEndpoint{
		data: map[ClustersKey]string{},
	}
}

type ClustersKey struct {
	ExternalId string
}

func (cs *ClustersEndpoint) ServeHTTP(w http.ResponseWriter, r *http.Request) {

	re := regexp.MustCompile(`'.*'`)

	switch r.Method {

	case http.MethodGet:
		cs.dataMux.RLock()
		defer cs.dataMux.RUnlock()

		w.WriteHeader(http.StatusOK)

		//for the mock server, we don't care about the expression itself,
		//we just want the cluster external id out of it
		//e.g.: external_id = 'a440b136-b2d6-406b-a884-fca2d62cd170'
		//get the id, with quotes
		search := r.URL.Query().Get("search")
		idFromSearch := re.FindStringSubmatch(search)

		//safeguard, when there's no cluster id in the search
		//string, we return an empty list of clusters
		if len(idFromSearch) == 0 {
			fmt.Fprintf(w, `{"items": []}`)
			return
		}

		//remove the quotes
		clusterExternalId := strings.Trim(idFromSearch[0], "'")

		//return always the same cluster id, regardless the external id
		//provided
		fmt.Fprintf(w,
			`{"items": [{"kind": "Cluster","id": "1ou","external_id": "%s"}]}`,
			clusterExternalId)

		log.Printf("%s %s:\n", r.URL.String(), r.Method)

	default:
		w.WriteHeader(http.StatusNotImplemented)
		return
	}
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
			fmt.Fprintln(w, `{"code":"not found","reason":"upgrade policy not found"}`)
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
