package serve

import (
	"encoding/json"
	"net/http"

	"github.com/gorilla/mux"

	"github.com/luisdavim/synthetic-checker/pkg/api"
	"github.com/luisdavim/synthetic-checker/pkg/checker"
	"github.com/luisdavim/synthetic-checker/pkg/checks"
	"github.com/luisdavim/synthetic-checker/pkg/config"
	"github.com/luisdavim/synthetic-checker/pkg/server"
)

func statusHandler(chkr *checker.CheckRunner, srv *server.Server, failStatus, degradedStatus int) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		statusCode := http.StatusOK
		checkStatus := chkr.GetStatus()
		if failStatus != http.StatusOK || degradedStatus != http.StatusOK {
			allFailed, anyFailed := checkStatus.Evaluate()
			if allFailed {
				statusCode = failStatus
			} else if anyFailed {
				statusCode = degradedStatus
			}
		}
		srv.JSONResponse(w, r, checkStatus, statusCode)
	}
}

func addCheckHandler(chkr *checker.CheckRunner) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		var check api.Check
		switch vars["type"] {
		case "http":
			var checkCfg config.HTTPCheck
			err := json.NewDecoder(r.Body).Decode(&checkCfg)
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			check, err = checks.NewHTTPCheck(vars["name"], checkCfg)
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
		case "dns":
			var checkCfg config.DNSCheck
			err := json.NewDecoder(r.Body).Decode(&checkCfg)
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			check, err = checks.NewDNSCheck(vars["name"], checkCfg)
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
		case "conn":
			var checkCfg config.ConnCheck
			err := json.NewDecoder(r.Body).Decode(&checkCfg)
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			check, err = checks.NewConnCheck(vars["name"], checkCfg)
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
		case "tls":
			var checkCfg config.TLSCheck
			err := json.NewDecoder(r.Body).Decode(&checkCfg)
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			check, err = checks.NewTLSCheck(vars["name"], checkCfg)
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
		case "k8s":
			var checkCfg config.K8sCheck
			err := json.NewDecoder(r.Body).Decode(&checkCfg)
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			check, err = checks.NewK8sCheck(vars["name"], checkCfg)
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
		case "grpc":
			var checkCfg config.GRPCCheck
			err := json.NewDecoder(r.Body).Decode(&checkCfg)
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			check, err = checks.NewGrpcCheck(vars["name"], checkCfg)
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
		default:
			http.Error(w, "unknown check type", http.StatusBadRequest)
			return
		}
		chkr.AddCheck(vars["name"], check)
	}
}

func deleteCheckHandler(chkr *checker.CheckRunner) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		chkr.DelCheck(vars["name"])
	}
}

func setRoutes(chkr *checker.CheckRunner, srv *server.Server, failStatus, degradedStatus int) {
	routes := server.Routes{
		"/": {
			Func:    statusHandler(chkr, srv, failStatus, degradedStatus),
			Methods: []string{http.MethodGet},
			Name:    "status",
		},
		"/checks/{type}/{name}": {
			Func:    addCheckHandler(chkr),
			Methods: []string{http.MethodPost, http.MethodPut},
			Name:    "add",
		},
		"/checks/{name}": {
			Func:    deleteCheckHandler(chkr),
			Methods: []string{http.MethodDelete},
			Name:    "delete",
		},
	}
	srv.WithRoutes(routes)
}
