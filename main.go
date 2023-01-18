package main

import (
	"flag"
	"log"
	"net/http"

	"k8s.io/klog/v2"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func main() {
	klog.InitFlags(nil)
	defer klog.Flush()
	flag.Set("v", "1")
	flag.Parse()

	reg := prometheus.NewRegistry()
	foo, err := NewCommitTimeCollector()
	if err != nil {
		klog.Errorf("can't find openshift cluster: %s", err)
		return
	}
	reg.MustRegister(foo)
	klog.Info("Running exporters...")
	http.Handle("/metrics", promhttp.HandlerFor(reg, promhttp.HandlerOpts{Registry: reg}))
	log.Fatal(http.ListenAndServe(":9101", nil))
}
