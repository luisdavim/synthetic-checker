# Synthetic Checker

The synthetic checker is a tool to run synthetic tests (checks) and track their statusses.
The tool supports the following types of checks:

- HTTP
- gRPC
- DNS
- Connection
- TLS/Certificate
- Kubernetes

More types of checks can be added in the future.

Checks can be executed periodically and asynchronously in the background.

## Development

To make development easier, a [Makefile](./Makefile) is provided, run `make` with no arguments to get a list of all available options.
To run all the linters, tests and build a binary, run `make build`
You can find the development docs in the [docs](./docs) folder.

## Usage

This tool can be used either as a cli to run the checks once (with optional retries) and get the results printed out as JSON.
Or as a service that periodically runs the checks and exposes the results through an API endpoint.
When running in service mode, the tool also exposes SLI metrics in the Prometheus format under the `/metrics` endpoint.
When running in the cli mode, the checks are stil executed in parallel and the configured `initialDelay` is stil used.
The cli mode can be used to run as an `init` container or as an `helm` pre or post hook as a pre-flight check or a post run validation. Or in a CI pipeline.

See the [usage docs](./usage) and check the `--help` flag on each sub-command to see all available options:

```console
$ go run main.go -h
A tool to run synthetic checks and report their results.

Usage:
  synthetic-checker [command]

Available Commands:
  check       Run the checks once and get an exit code
  completion  Generate the autocompletion script for the specified shell
  help        Help about any command
  serve       Run as a service

Flags:
  -c, --config string   config file (default is $HOME/.checks.yaml)
  -h, --help            help for synthetic-checker

Use "synthetic-checker [command] --help" for more information about a command.
```

To run the tool as a service:

```console
$ go run main.go start --help
Run as a service.

Usage:
  synthetic-checker serve [flags]

Aliases:
  serve, run, start

Flags:
  -C, --certFile string             File containing the x509 Certificate for HTTPS.
  -d, --debug                       Set log level to debug
  -D, --degraded-status-code int    HTTP status code to return when check check is failed (default 200)
  -F, --failed-status-code int      HTTP status code to return when all checks are failed (default 200)
  -h, --help                        help for serve
      --k8s-leader-election         Enable leader election, only works when running in k8s
  -K, --keyFile string              File containing the x509 private key for HTTPS.
      --leader-election-id string   set the leader election ID, defaults to POD_NAME or hostname
      --leader-election-ns string   set the leader election namespace, defaults to the current namespace
  -P, --pass string                 Set BasicAuth password for the http listener
  -p, --port int                    Port for the http listener (default 8080)
      --pretty-json                 Pretty print JSON responses
  -l, --request-limit int           Max requests per second per client allowed
  -s, --securePort int              Port for the HTTPS listener (default 8443)
  -S, --strip-slashes               Strip trailing slashes befofore matching routes
  -U, --user string                 Set BasicAuth user for the http listener
  -w, --watch-ingresses             Automatically setup checks for k8s ingresses, only works when running in k8s

Global Flags:
  -c, --config string   config file (default is $HOME/.checks.yaml)
```

## Configuration

By default the tool will look for a configuration file in one of the following locations:

- The location from where the tool was started: `./checks.yaml`
- The current user's home directory: `${HOME}/checks.yaml`
- The `/etcd` directory: `/etc/config/checks.yaml`

A sample configuration file can be found in [checks.yaml](./checks.yaml)

```yaml
---
httpChecks:
  stat503:
    url: https://httpstat.us/503
    interval: 10s
  stat200:
    url: https://httpstat.us/200
    interval: 10s
    initialDelay: 2s
dnsChecks:
  google:
    host: "www.google.com"
    interval: 15s
connChecks:
  cfDNS:
    address: "1.1.1.1:53"
    protocol: udp
tlsChecks:
  google:
    address: "www.google.com"
    expiryThreshold: 96h
k8sChecks:
  coredns: # a specific deployment
    kind: "Deployment.v1.apps"
    name: "coredns"
    namespace: "kube-system"
    interval: 20s
  nodes: # all the nodes
    kind: "Node.v1"
    interval: 30s
    initialDelay: 3s
  kube-api: # pods by label and namespace
    kind: "Pod.v1"
    namespace: "kube-system"
    labelSelector: "k8s-app=kube-apiserver"
    interval: 30s
    initialDelay: 4s
  monitoring-deployments: # all deployments in a namespace
    kind: "Deployment.v1.apps"
    namespace: "monitoring"
    interval: 30s
    initialDelay: 2s
```

### Informer

When running this tool as a service, you can configure it as an informer for syncing check configurations to upstream `synthetic-checker` instances.
This is particularly useful when running in Kubernetes and watching `Ingress` resources. It allows to dynamically generate checks for all `Ingress` resources (using the `--watch-ingresses` flag) on the local cluster and run the checks from outside the cluster, in one or more upstream instances.

Example configuration:

```yaml
informer:
  informOnly: true # when set to true, will prevent the checks from being executed in the local instance
  upstreams:
    - url: https://synthetic-checker.us.example.com
    - url: https://synthetic-checker.eu.example.com
```

### Watching ingress resources

When running as a service in a k8s cluster, the tool can also watch `Ingress` resources and automatically setup checks for them.
By default, the tool will setup DNS and connection checks for each ingress. It will check that all the host names resolve and will check if port 443 is reacheable on the ingress's LBs and that the TLS certFile is not about to expire.
You can annotate your `Ingress` resources to control the checks configuration.
No HTTP checks willl be create if both the endpoints and the configFrom annotations are missing.
If the `synthetic-checker/configFrom` annotation is set, and points to a valid secret, but the `synthetic-checker/endpoints` is not, an HTTP check will be set up with an empty endpoint.

```yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: sample-ingress
  annotations:
    nginx.ingress.kubernetes.io/server-alias: "test.example.com"
    synthetic-checker/skip: "false" # set to true to ignore this resource, defaults to false
    synthetic-checker/interval: "60s" # defaults to 30s
    synthetic-checker/ports: "80,443" # defaults to 443
    synthetic-checker/TLS: "false" # set this if all ports use TLS, defaults to false and only port 443 will use a TLS check
    synthetic-checker/noTLS: "false" # set this to disable TLS checks and use connection checks only, defaults to false
    synthetic-checker/endpoints: "/healthz,/readyz" # comma separated list of endpoints to build urls for HTTP checks
    synthetic-checker/configFrom: "cfgSecret" # name of a k8s secret, in the same namespace as the Ingress, with check configuration
spec:
  ingressClassName: nginx-example
  rules:
    - host: "foo.bar.com"
      http:
        paths:
          - path: /testpath
            pathType: Prefix
            backend:
              service:
                name: test
                port:
                  number: 80
```

You can keep some configuration in a K8s `Secret` and link it to an `Ingress` resource through the `synthetic-checker/configFrom` annotation, the expected format for that secret is as follows:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: cfgSecret
type: Opaque
stringData: # using stringData here just to make the config example more clear
  method: POST # HTTP method to use (optional)
  body: '{"foo": "bar", "some": "payload"}' # Body to use in the check requests (optional)
  foo: bar # any key that is not one of body or method is assumed to be an header to use in the check requests
  some: value # add as manay headers as needed
  Authorization: Basic YWxhZGRpbjpvcGVuc2VzYW1l # the main reason for using a secret is that you might need to set some auth headers
```

### HA modes

When running in Kubernetes, you have 2 options for running in HA mode.

- Running multiple independent instances, where each will execute its own checks.
  To use this mode set the `replicaCount` to any number higher than 1
- Running multiple instances with leader election, were the leader will execute the checks and the followers will sync the results from it.
  To use this mode set the `replicaCount` to any number higher than 1 and `k8sLeaderElection` to `true`

## Running as a service

You can run the service locally with docker or using the built binary directly. Or, you can run it in a Kubernetes cluster using the provided Helm chart.

Once the service has been started, you can try its API endpoints with curl.

Check the global status of all checks:

```console
$ curl -s http://localhost:8080/ | jq .
{
  "stat200-http": {
    "ok": true,
    "timestamp": "2022-12-24T01:16:54.554431Z",
    "duration": "437.32475ms",
    "contiguousFailures": 0,
    "timeOfFirstFailure": "0001-01-01T00:00:00Z"
  },
  "stat503-http": {
    "error": "Unexpected status code: '503' expected: '200'",
    "timestamp": "2022-12-24T01:16:54.554441Z",
    "duration": "438.6635ms",
    "contiguousFailures": 1,
    "timeOfFirstFailure": "2022-12-24T01:16:54.554441Z"
  }
}
```

Or check the metrics endpoint:

```console
$ curl -s http://localhost:8080/metrics | grep 'check_status_up'
# HELP check_status_up Status from the check
# TYPE check_status_up gauge
check_status_up{name="stat200"} 1
check_status_up{name="stat503"} 0
```

The following is an example of the metrics that are available for each check:

```console
# where the check name is stat200-http
$ curl -s http://localhost:8080/metrics | grep 'stat200-http'
# check_duration_ms is an histogram of the time it took run the check
check_duration_ms_bucket{name="stat200-http",le="5"} 0
check_duration_ms_bucket{name="stat200-http",le="10"} 0
check_duration_ms_bucket{name="stat200-http",le="25"} 0
check_duration_ms_bucket{name="stat200-http",le="50"} 0
check_duration_ms_bucket{name="stat200-http",le="100"} 0
check_duration_ms_bucket{name="stat200-http",le="250"} 0
check_duration_ms_bucket{name="stat200-http",le="500"} 4
check_duration_ms_bucket{name="stat200-http",le="1000"} 4
check_duration_ms_bucket{name="stat200-http",le="2500"} 4
check_duration_ms_bucket{name="stat200-http",le="5000"} 4
check_duration_ms_bucket{name="stat200-http",le="10000"} 4
check_duration_ms_bucket{name="stat200-http",le="+Inf"} 4
check_duration_ms_sum{name="stat200-http"} 1732
check_duration_ms_count{name="stat200-http"} 4
# check_status_total is a counter of the check result statusses, "success" or "error"
check_status_total{name="stat200-http",status="success"} 4
# check_status_up is a gauge indicating the check's last observed status, 1 success or 0 error
check_status_up{name="stat200-http"} 1
```

The API also allows adding or removing check configurations at runtime, check the integration tests [script](./scripts/test.sh) for some examples.

### Using go run

The simplest way to start the service is through the `go run` command:

```sh
go run main.go start
```

### Using Docker

You can run the service through Docker with the following commands:

```sh
docker build --rm --tag synthetic-checker
docker run -it --mount type=bind,source=${PWD}/checks.yaml,target=/etc/config/checks.yaml -p 0.0.0.0:8080:8080 synthetic-checker
```

note: You can also build the docker image using the provided `Makefile` with `make docker-build`

### In Kubernetes using Helm

You can run the service in a Kubernetes cluster using the provided Dockerfile and helm chart.

Build and publish the docker image:

```sh
make docker-build
make docker-release # add -e DOCKER_REGISTRY="my.reg.com" to override Docker registry
```

Create a helm values file with your configuration overrides, you can see an example in [charts/synthetic-checker/ci/with_checks.yaml](./charts/synthetic-checker/ci/with_checks.yaml),
Or have a look at the default [values](./charts/synthetic-checker/values.yaml) to see all the available options.

And deploy the service using the following command:

```sh
helm upgrade --install -n <target_namespace> -f <path/to/your/custom_values.yaml> synthetic-checker ./charts/synthetic-checker
```

### In local Kubernetes using helm and colima

If you don't have `colima` installed, have a look at [colima's GitHub page](https://github.com/abiosoft/colima)

Start colima with Kubernetes enabled:

```sh
colima start --kubernetes
```

Once colima is ready, build the docker image with it

```sh
make docker-build
```

Once the image is built, colima makes it available in the Kubernetes cluster and you can install the service using helm:

```sh
helm upgrade --install -f charts/synthetic-checker/ci/with_checks.yaml synthetic-checker ./charts/synthetic-checker
```

To test the service, you can use kubectl port forwarding:

```sh
kubectl port-forward svc/synthetic-checker 8080:80
```

And from another terminal window, tab or Tmux pane, you can reach the service with:

```sh
curl -s http://localhost:8080/
```

## Monitoring synthetic checkes with Prometheus

Install Prometheus:

```sh
kubectl create namespace monitoring
helm repo add prometheus-community https://prometheus-community.github.io/helm-charts
helm repo update
helm upgrade --namespace monitoring --install kube-stack-prometheus prometheus-community/kube-prometheus-stack --set prometheus-node-exporter.hostRootFsMount.enabled=false --set prometheus.prometheusSpec.podMonitorSelectorNilUsesHelmValues=false --set prometheus.prometheusSpec.probeSelectorNilUsesHelmValues=false --set prometheus.prometheusSpec.ruleSelectorNilUsesHelmValues=false --set prometheus.prometheusSpec.serviceMonitorSelectorNilUsesHelmValues=false --set thanosRuler.thanosRulerSpec.ruleSelectorNilUsesHelmValues=false
```

Update the synthetic-checker installation to include the Prometheus operator related resources:

```sh
helm upgrade --install -f charts/synthetic-checker/ci/with_checks.yaml -f charts/synthetic-checker/ci/with_prom_op.yaml synthetic-checker ./charts/synthetic-checker
```

Connecting to the Prometheus web UI:

```sh
kubectl port-forward --namespace monitoring svc/kube-stack-prometheus-kube-prometheus 9090:9090
```

Opening a browser tab on http://localhost:9090 shows the Prometheus web UI, the following URL will give you the status of your HTTP checks:

```text
http://localhost:9090/graph?g0.expr=check_status_up&g0.tab=0&g0.stacked=0&g0.show_exemplars=0&g0.range_input=1h
```

Connecting To Grafana:

The credentials to connect to the Grafana web interface are stored in a Kubernetes Secret and encoded in base64. We retrieve the username/password couple with these two commands:

```sh
kubectl get secret --namespace monitoring kube-stack-prometheus-grafana -o jsonpath='{.data.admin-user}' | base64 -d
kubectl get secret --namespace monitoring kube-stack-prometheus-grafana -o jsonpath='{.data.admin-password}' | base64 -d
```

We create the port-forward to Grafana with the following command:

```sh
kubectl port-forward --namespace monitoring svc/kube-stack-prometheus-grafana 8080:80
```

Open your browser, go to http://localhost:8080 and fill in previous credentials

A dashboard is included in the provided helm chart, search for "synthetic checks" in the Grafana UI.

![dashboard](./imgs/dash.png)
