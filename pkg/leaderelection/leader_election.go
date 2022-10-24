package leaderelection

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/rs/zerolog"
	coordinationv1client "k8s.io/client-go/kubernetes/typed/coordination/v1"
	corev1client "k8s.io/client-go/kubernetes/typed/core/v1"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/leaderelection"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
	konfig "sigs.k8s.io/controller-runtime/pkg/client/config"
)

const (
	inClusterNamespacePath = "/var/run/secrets/kubernetes.io/serviceaccount/namespace"
	lockName               = "synthetic-checker"
)

func getInClusterNamespace() (string, error) {
	// Check whether the namespace file exists.
	// If not, we are not running in cluster so can't guess the namespace.
	if _, err := os.Stat(inClusterNamespacePath); os.IsNotExist(err) {
		return "", fmt.Errorf("not running in-cluster, please specify LeaderElectionNamespace")
	} else if err != nil {
		return "", fmt.Errorf("error checking namespace file: %w", err)
	}

	// Load the namespace file and return its content
	namespace, err := os.ReadFile(inClusterNamespacePath)
	if err != nil {
		return "", fmt.Errorf("error reading namespace file: %w", err)
	}
	return string(namespace), nil
}

func NewResourceLock(id, namespace string) (resourcelock.Interface, error) {
	if id == "" {
		id = os.Getenv("POD_NAME")
	}
	if namespace == "" {
		var err error
		namespace, err = getInClusterNamespace()
		if err != nil {
			return nil, err
		}
	}

	config, err := konfig.GetConfig()
	if err != nil {
		return nil, err
	}

	rest.AddUserAgent(config, "leader-election")
	corev1Client, err := corev1client.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	coordinationClient, err := coordinationv1client.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	return resourcelock.New(resourcelock.LeasesResourceLock,
		namespace,
		lockName,
		corev1Client,
		coordinationClient,
		resourcelock.ResourceLockConfig{
			Identity: id,
		})
}

type LeaderElector struct {
	ID     string
	Leader string
	lock   resourcelock.Interface
	logger zerolog.Logger
}

func NewLeaderElector(id, namespace string) (*LeaderElector, error) {
	lock, err := NewResourceLock(id, namespace)
	if err != nil {
		return nil, err
	}
	logLevel := zerolog.InfoLevel
	return &LeaderElector{
		ID:     id,
		lock:   lock,
		logger: zerolog.New(os.Stderr).With().Timestamp().Str("name", "leaderElector").Logger().Level(logLevel),
	}, nil
}

func (l *LeaderElector) RunLeaderElection(ctx context.Context, run func(context.Context), sync func(leader string)) {
	done := make(chan struct{})
	leaderelection.RunOrDie(ctx, leaderelection.LeaderElectionConfig{
		Lock:            l.lock,
		ReleaseOnCancel: true,
		LeaseDuration:   15 * time.Second,
		RenewDeadline:   10 * time.Second,
		RetryPeriod:     2 * time.Second,
		Callbacks: leaderelection.LeaderCallbacks{
			OnStartedLeading: func(c context.Context) {
				l.logger.Info().Msg("I'm the leader")
				run(c)
			},
			OnStoppedLeading: func() {
				l.logger.Info().Msg("no longer the leader")
				os.Exit(1) // TODO: is this overkill?
			},
			OnNewLeader: func(currentID string) {
				l.logger.Info().Msgf("new leader is %s", currentID)
				if l.ID == currentID && l.Leader != "" {
					done <- struct{}{} // stop the sync
				} else {
					go func() {
						for {
							select {
							case <-done:
								return
							default:
								sync(currentID)
							}
							time.Sleep(9 * time.Second)
						}
					}()
				}
				l.Leader = currentID
			},
		},
	})
}
