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

	finalizerName := "synthetic_checker/finalizer"

	if ingress.DeletionTimestamp.IsZero() {
		if !controllerutil.ContainsFinalizer(ingress, finalizerName) {
			controllerutil.AddFinalizer(ingress, finalizerName)
			if err := r.Update(ctx, ingress); err != nil {
				return ctrl.Result{}, err
			}
		}
	} else {
		// The object is being deleted
		if controllerutil.ContainsFinalizer(ingress, finalizerName) {
			if err := r.cleanup(ingress); err != nil {
				return ctrl.Result{}, err
			}
			controllerutil.RemoveFinalizer(ingress, finalizerName)
			if err := r.Update(ctx, ingress); err != nil {
				return ctrl.Result{}, err
			}
		}
		return ctrl.Result{}, nil
	}

	if err := r.setup(ingress); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{RequeueAfter: time.Hour}, nil
}

func (r *IngressReconciler) setup(ingress *netv1.Ingress) error {
	hosts := getHosts(ingress)

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

	return nil
}

func (r *IngressReconciler) cleanup(ingress *netv1.Ingress) error {
	hosts := getHosts(ingress)

	for _, host := range hosts {
		name := host + "-dns"
		r.Checker.DelCheck(name)
	}
	return nil
}

func getHosts(ingress *netv1.Ingress) []string {
	var hosts []string
	for _, rule := range ingress.Spec.Rules {
		if rule.Host != "" {
			hosts = append(hosts, rule.Host)
		}
	}
	if aliases, ok := ingress.GetAnnotations()["nginx.ingress.kubernetes.io/server-alias"]; ok {
		for _, host := range strings.Split(aliases, ",") {
			hosts = append(hosts, strings.TrimSpace(host))
		}
	}

	for _, status := range ingress.Status.LoadBalancer.Ingress {
		if status.Hostname != "" {
			hosts = append(hosts, status.Hostname)
		}
	}

	return hosts
}
