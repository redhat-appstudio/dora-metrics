// Package api provides ArgoCD application watching functionality.
// It implements a worker pool-based event processing system that can handle
// high-volume ArgoCD application events efficiently.
package api

import (
	"context"
	"sync"
	"time"

	"github.com/redhat-appstudio/dora-metrics/pkg/logger"

	"github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	argocd "github.com/argoproj/argo-cd/v2/pkg/client/clientset/versioned"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
)

// ArgoCDWatcher implements an ArgoCD application watcher.
// It watches ArgoCD applications in configured namespaces and uses a worker pool
// pattern to process events concurrently and efficiently.
type ArgoCDWatcher struct {
	client       Client
	eventHandler EventHandler
	parser       ApplicationParser
	workers      int
	eventCh      chan watch.Event
	stopCh       chan struct{}
	wg           sync.WaitGroup
}

// NewArgoCDWatcher creates a new ArgoCD watcher instance.
// It takes a client, event handler, parser, and number of workers as parameters.
func NewArgoCDWatcher(
	client Client,
	eventHandler EventHandler,
	parser ApplicationParser,
	workers int,
) Monitor {
	return &ArgoCDWatcher{
		client:       client,
		eventHandler: eventHandler,
		parser:       parser,
		workers:      workers,
		eventCh:      make(chan watch.Event, 10000),
		stopCh:       make(chan struct{}),
	}
}

// Start begins watching for ArgoCD application events.
func (w *ArgoCDWatcher) Start(ctx context.Context) error {
	logger.Info("Starting ArgoCD application watcher")

	// Start worker goroutines
	for i := 0; i < w.workers; i++ {
		w.wg.Add(1)
		go w.eventWorker(ctx, i)
	}

	// Start the watch loop
	w.wg.Add(1)
	go w.watchLoop(ctx)

	return nil
}

// Stop stops the watcher.
func (w *ArgoCDWatcher) Stop() {
	logger.Info("Stopping ArgoCD application watcher")
	close(w.stopCh)
	w.wg.Wait()
	close(w.eventCh)
}

// eventWorker processes events from the event channel.
func (w *ArgoCDWatcher) eventWorker(ctx context.Context, workerID int) {
	defer w.wg.Done()

	for {
		select {
		case event, ok := <-w.eventCh:
			if !ok {
				return
			}

			if err := w.handleEvent(ctx, event); err != nil {
				logger.Errorf("Worker %d failed to handle event: %v", workerID, err)
			}

		case <-ctx.Done():
			return

		case <-w.stopCh:
			return
		}
	}
}

// watchLoop sets up Kubernetes watches for ArgoCD applications.
func (w *ArgoCDWatcher) watchLoop(ctx context.Context) {
	defer w.wg.Done()

	logger.Info("Starting ArgoCD application watch loop")

	// Get the ArgoCD client
	argocdClient := w.client.GetArgoCDClient()
	if argocdClient == nil {
		logger.Error("ArgoCD client is nil, cannot start watching")
		return
	}

	// Get namespaces to watch
	namespaces := w.client.GetNamespaces()
	if len(namespaces) == 0 {
		logger.Warn("No namespaces configured for watching")
		return
	}

	// Start watching each namespace
	for _, namespace := range namespaces {
		go w.watchNamespace(ctx, argocdClient, namespace)
	}

	// Wait for context cancellation or stop signal
	select {
	case <-ctx.Done():
		logger.Info("Watch loop stopped due to context cancellation")
	case <-w.stopCh:
		logger.Info("Watch loop stopped due to stop signal")
	}
}

// watchNamespace watches ArgoCD applications in a specific namespace.
func (w *ArgoCDWatcher) watchNamespace(ctx context.Context, argocdClient *argocd.Clientset, namespace string) {
	logger.Infof("Starting watch for ArgoCD applications in namespace: %s", namespace)

	// Set up the watch with retry logic
	for {
		select {
		case <-ctx.Done():
			logger.Infof("Stopping watch for namespace %s due to context cancellation", namespace)
			return
		case <-w.stopCh:
			logger.Infof("Stopping watch for namespace %s due to stop signal", namespace)
			return
		default:
			logger.Infof("Creating watch for namespace: %s", namespace)

			// First, test if we can list applications in the namespace
			apps, err := argocdClient.ArgoprojV1alpha1().Applications(namespace).List(ctx, metav1.ListOptions{})
			if err != nil {
				logger.Errorf("Failed to list applications in namespace %s: %v", namespace, err)
				time.Sleep(5 * time.Second) // Retry after 5 seconds
				continue
			}
			logger.Infof("Found %d applications in namespace %s", len(apps.Items), namespace)

			// Create the watch with timeout handling
			watchInterface, err := argocdClient.ArgoprojV1alpha1().Applications(namespace).Watch(ctx, metav1.ListOptions{
				Watch:          true,
				TimeoutSeconds: int64Ptr(300), // 5 minutes timeout
				// Watch all applications in the namespace
			})
			if err != nil {
				logger.Errorf("Failed to create watch for namespace %s: %v", namespace, err)
				time.Sleep(5 * time.Second) // Retry after 5 seconds
				continue
			}

			logger.Infof("Watch created successfully for namespace: %s", namespace)

			// Process watch events
			w.processWatchEvents(ctx, watchInterface, namespace)

			// If we get here, the watch ended, so we'll retry
			logger.Warnf("Watch ended for namespace %s, retrying...", namespace)
			time.Sleep(1 * time.Second)
		}
	}
}

// processWatchEvents processes events from a watch interface.
func (w *ArgoCDWatcher) processWatchEvents(ctx context.Context, watchInterface watch.Interface, namespace string) {
	defer watchInterface.Stop()

	logger.Infof("Starting to process watch events for namespace: %s", namespace)
	eventCount := 0

	for {
		select {
		case event, ok := <-watchInterface.ResultChan():
			if !ok {
				logger.Warnf("Watch channel closed for namespace %s (processed %d events)", namespace, eventCount)
				return
			}

			eventCount++

			// Send event to the event channel for processing by workers
			select {
			case w.eventCh <- event:
				// Event sent successfully
			case <-ctx.Done():
				return
			case <-w.stopCh:
				return
			default:
				// Channel is full, log warning but continue
				logger.Warnf("Event channel is full, dropping event for namespace %s", namespace)
			}

		case <-ctx.Done():
			logger.Infof("Context cancelled while processing events for namespace %s (processed %d events)", namespace, eventCount)
			return
		case <-w.stopCh:
			logger.Infof("Stop signal received while processing events for namespace %s (processed %d events)", namespace, eventCount)
			return
		}
	}
}

// handleEvent processes a single watch event.
func (w *ArgoCDWatcher) handleEvent(ctx context.Context, event watch.Event) error {
	// Type assert to get the application
	app, ok := event.Object.(*v1alpha1.Application)
	if !ok {
		logger.Debugf("Received non-application event: %T", event.Object)
		return nil
	}

	// Handle the event (filtering is done in the event processor)
	return w.eventHandler.HandleEvent(ctx, event, app)
}

// int64Ptr returns a pointer to an int64 value
func int64Ptr(i int64) *int64 { return &i }
