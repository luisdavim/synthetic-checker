package ingresswatcher

import (
	"context"
	"strings"
	"time"

	netv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/luisdavim/synthetic-checker/pkg/checker"
	"github.com/luisdavim/synthetic-checker/pkg/checks"
	"github.com/luisdavim/synthetic-checker/pkg/config"
)

const (
	finalizerName = "synthetic-checker/finalizer"
	defaultLBPort = ":443"
)

// IngressReconciler reconciles a Ingress object
type IngressReconciler struct {
	client.Client
	Scheme  *runtime.Scheme
	Checker *checker.CheckRunner
}

func (r *IngressReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&netv1.Ingress{}).
		Complete(r)
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
			return ctrl.Result{RequeueAfter: time.Second}, nil
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

	log.Info("setting up checks for ingress")
	if err := r.setup(ingress); err != nil {
		log.Error(err, "failed to setup checks for ingress")
		return ctrl.Result{}, err
	}

	return ctrl.Result{RequeueAfter: time.Hour}, nil
}

func (r *IngressReconciler) setup(ingress *netv1.Ingress) error {
	hosts := getHosts(ingress)

	// setup DNS checks for all ingress Hostnames
	for i, host := range hosts {
		name := host + "-dns"
		check, err := checks.NewDNSCheck(name,
			config.DNSCheck{
				Host: host,
				BaseCheck: config.BaseCheck{
					InitialDelay: time.Duration(i) + 1*time.Second,
				},
			})
		if err != nil {
			return err
		}
		r.Checker.AddCheck(name, check)
	}

	// setup connection checks for all ingress LBs
	for i, lb := range getLBs(ingress) {
		name := lb + "-conn"
		check, err := checks.NewConnCheck(name,
			config.ConnCheck{
				Address: lb + defaultLBPort,
				BaseCheck: config.BaseCheck{
					InitialDelay: time.Duration(i) + 1*time.Second,
				},
			})
		if err != nil {
			return err
		}
		r.Checker.AddCheck(name, check)
	}

	return nil
}

func (r *IngressReconciler) cleanup(ingress *netv1.Ingress) error {
	for _, host := range getHosts(ingress) {
		name := host + "-dns"
		r.Checker.DelCheck(name)
	}

	for _, lb := range getLBs(ingress) {
		name := lb + "-conn"
		r.Checker.DelCheck(name)
	}

	return nil
}

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

	if aliases, ok := ingress.GetAnnotations()["nginx.ingress.kubernetes.io/server-alias"]; ok {
		for _, host := range strings.Split(aliases, ",") {
			host = strings.TrimSpace(host)
			if _, ok := found[host]; !ok {
				found[host] = struct{}{}
				hosts = append(hosts, host)
			}
		}
	}

	return hosts
}

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
