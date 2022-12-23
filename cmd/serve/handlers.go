package serve

import (
	"fmt"
	"io"
	"net/http"

	"github.com/gorilla/mux"
	"sigs.k8s.io/yaml"

	"github.com/luisdavim/synthetic-checker/pkg/checker"
	"github.com/luisdavim/synthetic-checker/pkg/config"
	"github.com/luisdavim/synthetic-checker/pkg/server"
)

const cfgTpl = `{"%sChecks": {%q: %s}}`

func statusHandler(chkr *checker.Runner, srv *server.Server, failStatus, degradedStatus int) http.HandlerFunc {
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

func addCheckHandler(chkr *checker.Runner) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		if r.Method == http.MethodDelete {
			name := vars["name"]
			if t, ok := vars["type"]; ok {
				name += "-" + t
			}
			chkr.DelCheck(name)
		}
		b, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		checkCfg := []byte(fmt.Sprintf(cfgTpl, vars["type"], vars["name"], string(b)))
		var cfg config.Config
		if err := yaml.Unmarshal(checkCfg, &cfg); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if err := chkr.AddFromConfig(cfg, true); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
	}
}

func deleteCheckHandler(chkr *checker.Runner) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		chkr.DelCheck(vars["name"])
	}
}

func setRoutes(chkr *checker.Runner, srv *server.Server, failStatus, degradedStatus int) {
	routes := server.Routes{
		"/": {
			Func:    statusHandler(chkr, srv, failStatus, degradedStatus),
			Methods: []string{http.MethodGet},
			Name:    "status",
		},
		"/checks/{type}/{name}": {
			Func:    addCheckHandler(chkr),
			Methods: []string{http.MethodPost, http.MethodPut, http.MethodDelete},
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
