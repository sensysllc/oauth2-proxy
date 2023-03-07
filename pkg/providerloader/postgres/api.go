package postgres

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"

	"github.com/gorilla/mux"
	"github.com/oauth2-proxy/oauth2-proxy/v7/pkg/apis/options"
	"github.com/oauth2-proxy/oauth2-proxy/v7/providers"
)

type API struct {
	rs   ConfigStore
	conf options.API
}

func NewAPI(conf options.API, rs *RedisStore, proxyPrefix string) error {
	r := mux.NewRouter()
	api := API{
		rs:   rs,
		conf: conf,
	}
	var pathPrefix = proxyPrefix

	if conf.PathPrefix != "" {
		pathPrefix = conf.PathPrefix
	}

	r2 := r.PathPrefix(pathPrefix).Subrouter()
	r2.HandleFunc("/provider", api.CreateHandler).Methods("POST")
	r2.HandleFunc("/provider", api.UpdateHandler).Methods("PUT")
	r2.HandleFunc("/provider/{id}", api.GetHandler).Methods("GET")
	r2.HandleFunc("/provider/{id}", api.DeleteHandler).Methods("DELETE")

	server := &http.Server{
		Handler:           r,
		Addr:              conf.Host + ":" + strconv.Itoa(conf.Port),
		ReadHeaderTimeout: conf.Timeout,
	}

	go func() {
		err := server.ListenAndServe()
		if err != nil && errors.Is(err, http.ErrServerClosed) {
			log.Fatal("server is closed invalid config")
		}
	}()

	return nil

}
func (api *API) CreateHandler(rw http.ResponseWriter, req *http.Request) {
	rw.Header().Set("Content-Type", "json")

	id, data, err := api.validateProviderConfig(req)
	if err != nil {
		rw.WriteHeader(http.StatusBadRequest)
		fmt.Fprint(rw, err)
		return
	}

	err = api.rs.Create(req.Context(), id, data)
	if err != nil {
		rw.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(rw, err)
		return
	}

	rw.WriteHeader(http.StatusCreated)
}

func (api *API) GetHandler(rw http.ResponseWriter, req *http.Request) {
	rw.Header().Set("Content-Type", "json")
	vars := mux.Vars(req)

	id := vars["id"]

	providerConf, err := api.rs.Get(req.Context(), id)
	if err != nil {
		rw.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(rw, err)
		return
	}

	rw.WriteHeader(http.StatusOK)
	fmt.Fprint(rw, providerConf)
}

func (api *API) DeleteHandler(rw http.ResponseWriter, req *http.Request) {
	rw.Header().Set("Content-Type", "json")

	vars := mux.Vars(req)

	id := vars["id"]

	err := api.rs.Delete(req.Context(), id)
	if err != nil {
		rw.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(rw, err)
		return
	}

	rw.WriteHeader(http.StatusNoContent)
}

func (api *API) UpdateHandler(rw http.ResponseWriter, req *http.Request) {
	rw.Header().Set("Content-Type", "json")

	id, data, err := api.validateProviderConfig(req)
	if err != nil {
		rw.WriteHeader(http.StatusBadRequest)
		fmt.Fprint(rw, err)
		return

	}

	err = api.rs.Update(req.Context(), id, data)
	if err != nil {
		rw.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(rw, err)
		return
	}

	rw.WriteHeader(http.StatusAccepted)

}

func (api *API) validateProviderConfig(req *http.Request) (string, []byte, error) {
	var body []byte
	var err error
	body, err = io.ReadAll(req.Body)
	defer req.Body.Close()

	if err != nil {
		return "", nil, fmt.Errorf("error while reading request body. %v", err)
	}

	var providerConf *options.Provider

	err = json.Unmarshal(body, &providerConf)
	if err != nil {
		return "", nil, fmt.Errorf("error while decoding JSON. %v", err)
	}

	if providerConf.ID == "" {
		return "", nil, fmt.Errorf("provider ID is not provided")
	}

	_, err = providers.NewProvider(*providerConf)
	if err != nil {
		return "", nil, fmt.Errorf("invalid provider configuration: %v", err)
	}

	data, err := json.Marshal(providerConf)
	if err != nil {
		return "", nil, fmt.Errorf("error in marshalling")
	}

	return providerConf.ID, data, nil
}
