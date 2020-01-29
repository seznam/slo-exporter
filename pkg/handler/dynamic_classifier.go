package handler

import (
	"fmt"
	"net/http"

	"github.com/gorilla/mux"
	"gitlab.seznam.net/sklik-devops/slo-exporter/pkg/dynamic_classifier"
)

type DynamicClassifierHandler struct {
	classifier *dynamic_classifier.DynamicClassifier
}

func NewDynamicClassifierHandler(c *dynamic_classifier.DynamicClassifier) *DynamicClassifierHandler {
	return &DynamicClassifierHandler{
		classifier: c,
	}
}

func (dc *DynamicClassifierHandler) DumpCSV(w http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	matcherType := vars["matcher"]
	w.Header().Set("Content-Type", "text/csv")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment;filename=%s.csv", matcherType))
	err := dc.classifier.DumpCSV(w, matcherType)
	if err != nil {
		errorsTotal.WithLabelValues(err.Error()).Inc()
		http.Error(w, "Failed to dump matcher '"+matcherType+"': "+err.Error(), http.StatusInternalServerError)
	}

}
