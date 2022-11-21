package ingresswatcher

import (
	"context"
	"strconv"
	"strings"
	"time"

	netv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	"github.com/luisdavim/synthetic-checker/pkg/api"
	"github.com/luisdavim/synthetic-checker/pkg/checker"
	"github.com/luisdavim/synthetic-checker/pkg/checks"
	"github.com/luisdavim/synthetic-checker/pkg/config"
)

const (
	finalizerName       = "synthetic-checker/finalizer"
	skipAnnotation      = "synthetic-checker/skip"
	tlsAnnotation       = "synthetic-checker/TLS"
	noTLSAnnotation     = "synthetic-checker/noTLS"
	portsAnnotation     = "synthetic-checker/ports"
	intervalAnnotation  = "synthetic-checker/interval"
	endpointsAnnotation = "synthetic-checker/endpoints"
	defaultLBPort       = ":443"
)

// TODO: allow the user to extend this list
var additionalHostsAnnotations = []string{
	"nginx.ingress.kubernetes.io/server-alias",
	"external-dns.alpha.kubernetes.io/hostname",
	"external-dns.alpha.kubernetes.io/internal-hostname",
	"dns.alpha.kubernetes.io/external",
	"dns.alpha.kubernetes.io/internal",
}

// IngressReconciler reconciles a Ingress object
type IngressReconciler struct {
	client.Client
	Scheme  *runtime.Scheme
	Checker *checker.CheckRunner
}

func (r *IngressReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&netv1.Ingress{}).
		WithEventFilter(predicates()).
		Complete(r)
}

// predicates will filter events for ingresses that haven't changed
// or are annotated to be skipped
func predicates() predicate.Predicate {
	p := predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			annotations := e.Object.GetAnnotations()
			skip, _ := strconv.ParseBool(annotations[skipAnnotation])
			return !skip
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			newAnnotations := e.ObjectNew.GetAnnotations()
			skip, _ := strconv.ParseBool(newAnnotations[skipAnnotation])
			if !skip {
				return true
			}
			oldAnnotations := e.ObjectOld.GetAnnotations()
			if s, _ := strconv.ParseBool(oldAnnotations[skipAnnotation]); !s {
				// cleanup needed
				return true
			}
			return !skip
		},
		DeleteFunc: func(e event.DeleteEvent) bool {
			return true
		},
		GenericFunc: func(e event.GenericEvent) bool {
			return true
		},
	}

	return predicate.And(predicate.Or(predicate.GenerationChangedPredicate{}, predicate.AnnotationChangedPredicate{}), p)
}

//+kubebuilder:rbac:groups=networking.k8s.io,resources=ingresss,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=networking.k8s.io,resources=ingresss/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=networking.k8s.io,resources=ingresss/finalizers,verbs=update

func (r *IngressReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := ctrl.Log.WithName("controller").WithValues("ingress", req.NamespacedName)

	ingress := &netv1.Ingress{}
	if err := r.Get(ctx, req.NamespacedName, ingress); err != nil {
		log.Error(err, "unable to fetch Ingress")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if ingress.DeletionTimestamp.IsZero() {
		if !controllerutil.ContainsFinalizer(ingress, finalizerName) {
			controllerutil.AddFinalizer(ingress, finalizerName)
			if err := r.Update(ctx, ingress); err != nil {
				log.Error(err, "failed to add finalizer")
				return ctrl.Result{}, err
			}
			// no need to exit here the predicates will filter the finalizer update event
			// return ctrl.Result{RequeueAfter: time.Second}, nil
		}
	} else {
		// The object is being deleted
		if controllerutil.ContainsFinalizer(ingress, finalizerName) {
			if err := r.cleanup(ingress); err != nil {
				log.Error(err, "failed to cleanup checks for ingress")
				return ctrl.Result{}, err
			}
			controllerutil.RemoveFinalizer(ingress, finalizerName)
			if err := r.Update(ctx, ingress); err != nil {
				log.Error(err, "failed to remove finalizer")
				return ctrl.Result{}, err
			}
		}
		return ctrl.Result{}, nil
	}

	if v, ok := ingress.Annotations[skipAnnotation]; ok {
		// skip annotation was added or changed from false to true
		if skip, _ := strconv.ParseBool(v); skip {
			err := r.cleanup(ingress)
			return ctrl.Result{}, err
		}
	}

	log.Info("setting up checks for ingress")
	if err := r.setup(ingress); err != nil {
		log.Error(err, "failed to setup checks for ingress")
		return ctrl.Result{}, err
	}

	return ctrl.Result{RequeueAfter: time.Hour}, nil
}

func (r *IngressReconciler) setup(ingress *netv1.Ingress) error {
	var interval time.Duration

	if i, ok := ingress.Annotations[intervalAnnotation]; ok {
		interval, _ = time.ParseDuration(i)
	}

	hosts := getHosts(ingress)
	ports := getPorts(ingress)

	// setup DNS checks for all ingress Hostnames
	if err := r.setDNSChecks(hosts, ports, interval); err != nil {
		return err
	}

	// setup connection checks for all ingress LBs
	lbs := getLBs(ingress)
	tls, _ := strconv.ParseBool(ingress.Annotations[tlsAnnotation])
	noTLS, _ := strconv.ParseBool(ingress.Annotations[noTLSAnnotation])
	if err := r.setConnChecks(lbs, ports, hosts, tls, noTLS, interval); err != nil {
		return err
	}

	// setup http checks
	endpoints := getEndpoints(ingress)
	if err := r.setHTTPChecks(hosts, ports, endpoints, interval); err != nil {
		return err
	}

	return nil
}

func (r *IngressReconciler) setDNSChecks(hosts, ports []string, interval time.Duration) error {
	for i, host := range hosts {
		name := host + "-dns"
		check, err := checks.NewDNSCheck(name,
			config.DNSCheck{
				Host: host,
				BaseCheck: config.BaseCheck{
					InitialDelay: time.Duration(i) + 1*time.Second,
					Interval:     interval,
				},
			})
		if err != nil {
			return err
		}
		r.Checker.AddCheck(name, check)
	}

	return nil
}

func (r *IngressReconciler) setConnChecks(lbs, ports, hosts []string, tls, noTLS bool, interval time.Duration) error {
	for i, lb := range lbs {
		for _, port := range ports {
			lb = lb + port
			name := lb + "-conn"
			var (
				err   error
				check api.Check
			)
			if !noTLS && (port == ":443" || tls) {
				name = lb + "-tls"
				check, err = checks.NewTLSCheck(name,
					config.TLSCheck{
						Address:             lb,
						HostNames:           hosts,
						InsecureSkipVerify:  true,
						SkipChainValidation: true,
						BaseCheck: config.BaseCheck{
							InitialDelay: time.Duration(i) + 1*time.Second,
							Interval:     interval,
						},
					})
			} else {
				check, err = checks.NewConnCheck(name,
					config.ConnCheck{
						Address: lb,
						BaseCheck: config.BaseCheck{
							InitialDelay: time.Duration(i) + 1*time.Second,
							Interval:     interval,
						},
					})
			}
			if err != nil {
				return err
			}
			r.Checker.AddCheck(name, check)
		}
	}

	return nil
}

func (r *IngressReconciler) setHTTPChecks(hosts, ports, endpoints []string, interval time.Duration) error {
	if len(endpoints) == 0 {
		return nil
	}

	for i, host := range hosts {
		for _, port := range ports {
			for _, endpoint := range endpoints {
				url := strings.ReplaceAll(host, "*", "check") + port + endpoint
				scheme := "https://"
				if strings.HasPrefix(port, ":80") {
					scheme = "http://"
				}
				name := url + "-http"
				url = scheme + url
				check, err := checks.NewHTTPCheck(name,
					config.HTTPCheck{
						URL: url,
						BaseCheck: config.BaseCheck{
							InitialDelay: time.Duration(i) + 1*time.Second,
							Interval:     interval,
						},
					})
				if err != nil {
					return err
				}
				r.Checker.AddCheck(name, check)
			}
		}
	}

	return nil
}

func (r *IngressReconciler) cleanup(ingress *netv1.Ingress) error {
	hosts := getHosts(ingress)
	ports := getPorts(ingress)

	// cleanup DNS checks
	for _, host := range hosts {
		name := host + "-dns"
		r.Checker.DelCheck(name)
	}

	// cleanup connection checks
	tls, _ := strconv.ParseBool(ingress.Annotations[tlsAnnotation])
	noTLS, _ := strconv.ParseBool(ingress.Annotations[noTLSAnnotation])

	for _, lb := range getLBs(ingress) {
		for _, port := range ports {
			name := lb + port + "-conn"
			if !noTLS && (port == ":443" || tls) {
				name = lb + port + "-tls"
			}
			r.Checker.DelCheck(name)
		}
	}

	endpoints := getEndpoints(ingress)
	if len(endpoints) == 0 {
		return nil
	}

	// cleanup http checks
	for _, host := range hosts {
		for _, port := range ports {
			for _, endpoint := range endpoints {
				url := strings.ReplaceAll(host, "*", "check") + port + endpoint
				name := url + "-http"
				r.Checker.DelCheck(name)
			}
		}
	}

	return nil
}

func getEndpoints(ingress *netv1.Ingress) []string {
	var endpoints []string
	if e, ok := ingress.Annotations[endpointsAnnotation]; ok {
		for _, endpoint := range strings.Split(e, ",") {
			endpoint = strings.TrimSpace(endpoint)
			if endpoint != "" {
				if !strings.HasPrefix(endpoint, "/") {
					endpoint = "/" + endpoint
				}
				endpoints = append(endpoints, endpoint)
			}
		}
	}

	return endpoints
}

// getPorts returns the list of ports to check by inspectin the resource's annotations
func getPorts(ingress *netv1.Ingress) []string {
	var ports []string
	if ps, ok := ingress.Annotations[portsAnnotation]; ok {
		for _, port := range strings.Split(ps, ",") {
			port = strings.TrimSpace(port)
			if port == "" {
				continue
			}
			if !strings.HasPrefix(port, ":") {
				port = ":" + port
			}
			ports = append(ports, port)
		}
	}
	if len(ports) == 0 {
		ports = append(ports, defaultLBPort)
	}

	return ports
}

// getHosts returns the list of hosts to check by inspectin the resource's spec and annotations
func getHosts(ingress *netv1.Ingress) []string {
	var hosts []string
	found := make(map[string]struct{})

	for _, rule := range ingress.Spec.Rules {
		if rule.Host == "" {
			continue
		}
		if _, ok := found[rule.Host]; !ok {
			found[rule.Host] = struct{}{}
			hosts = append(hosts, rule.Host)
		}
	}

	for _, annotation := range additionalHostsAnnotations {
		if aliases, ok := ingress.Annotations[annotation]; ok {
			for _, host := range strings.Split(aliases, ",") {
				host = strings.TrimSpace(host)
				if _, ok := found[host]; !ok && host != "" {
					found[host] = struct{}{}
					hosts = append(hosts, host)
				}
			}
		}
	}

	return hosts
}

// getLBs returns the list of LBs to check by inspectin the resource's status
func getLBs(ingress *netv1.Ingress) []string {
	var hosts []string
	found := make(map[string]struct{})

	for _, status := range ingress.Status.LoadBalancer.Ingress {
		if status.Hostname == "" {
			continue
		}
		if _, ok := found[status.Hostname]; !ok {
			found[status.Hostname] = struct{}{}
			hosts = append(hosts, status.Hostname)
		}
	}

	return hosts
}
