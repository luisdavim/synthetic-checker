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

func newResourceLock(id, namespace string) (resourcelock.Interface, error) {
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
	logLevel := zerolog.InfoLevel
	logger := zerolog.New(os.Stderr).With().Timestamp().Str("name", "leaderElector").Logger().Level(logLevel)
	if id == "" {
		id = os.Getenv("POD_NAME")
	}
	if id == "" {
		var err error
		id, err = os.Hostname()
		if err != nil {
			return nil, err
		}
	}
	if namespace == "" {
		var err error
		namespace, err = getInClusterNamespace()
		if err != nil {
			return nil, err
		}
	}
	logger.Info().Msgf("setting up leader election, ID: %s, namespace: %s", id, namespace)
	lock, err := newResourceLock(id, namespace)
	if err != nil {
		return nil, err
	}
	return &LeaderElector{
		ID:     id,
		lock:   lock,
		logger: logger,
	}, nil
}

func (l *LeaderElector) RunLeaderElection(ctx context.Context, run func(context.Context), sync func(leader string)) {
	l.logger.Info().Msg("starting leader election runner")
	done := make(chan struct{})
	leaderelection.RunOrDie(ctx, leaderelection.LeaderElectionConfig{
		Lock:            l.lock,
		ReleaseOnCancel: true,
		LeaseDuration:   15 * time.Second,
		RenewDeadline:   10 * time.Second,
		RetryPeriod:     2 * time.Second,
		Callbacks: leaderelection.LeaderCallbacks{
			OnStartedLeading: func(c context.Context) {
				l.logger.Info().Msg("starting main loop")
				run(c)
			},
			OnStoppedLeading: func() {
				l.logger.Info().Msg("no longer the leader")
				os.Exit(1) // TODO: is this overkill?
			},
			OnNewLeader: func(currentID string) {
				l.logger.Info().Str("leader", l.Leader).Msgf("new leader is %s, I am %s", currentID, l.ID)
				if l.ID == currentID {
					l.logger.Info().Msg("I am the leader")
					if l.Leader != "" { // if the sync never started, no need to stop it
						l.logger.Info().Msg("stopping sync loop")
						done <- struct{}{} // stop the sync
					}
				} else {
					l.logger.Info().Str("leader", currentID).Msg("starting sync loop")
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
