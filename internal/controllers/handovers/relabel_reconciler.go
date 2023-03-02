package handovers

import (
	"context"
	"fmt"
	"sort"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/client-go/util/jsonpath"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"package-operator.run/apis/coordination/v1alpha1"
	coordinationv1alpha1 "package-operator.run/apis/coordination/v1alpha1"
	"package-operator.run/package-operator/internal/controllers"
	"package-operator.run/package-operator/internal/probing"
)

// Executes the re-label strategy.
type relabelReconciler struct {
	client       client.Writer
	dynamicCache client.Reader
}

func newRelabelReconciler(client client.Writer, dynamicCache client.Reader) *relabelReconciler {
	return &relabelReconciler{
		client:       client,
		dynamicCache: dynamicCache,
	}
}

func (r *relabelReconciler) Reconcile(
	ctx context.Context, handover genericHandover) (ctrl.Result, error) {
	if handover.GetStrategyType() != v1alpha1.HandoverStrategyRelabel {
		// different strategy
		return ctrl.Result{}, nil
	}

	gvk, objType, objListType := controllers.UnstructuredFromTargetAPI(handover.GetTargetAPI())
	relabelSpec := handover.GetRelabelStrategy()

	probe := probing.ParseProbes(ctx, handover.GetAvailabilityProbes())

	// Handle objects already in the processing set:
	stillProcessing, err := r.handleProcessingSet(ctx, relabelSpec,
		objType, probe, handover.GetProcessing())
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("handle processing set: %w", err)
	}
	handover.SetProcessing(stillProcessing)

	// List all objects
	objects, err := r.listAllObjects(ctx, handover, objListType, gvk)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("list all objects: %w", err)
	}

	// split total into old and new
	groups := groupByLabelValues(
		objects, relabelSpec.LabelKey,
		relabelSpec.ToValue,
		relabelSpec.InitialValue,
	)
	newObjs := groups[0]
	oldObjs := groups[1]

	// count totals
	totalStats := countObjectStatus(objects, probe, newObjs)
	unavailable := int(totalStats.Found) - int(totalStats.Available)

	// Partition stats
	var partitionStats []coordinationv1alpha1.HandoverPartitionStatus
	partition := handover.GetPartitionSpec()
	if partition != nil {
		partitionLabelValues := labelValues(objects, partition.LabelKey)
		switch partition.Order.Type {
		case coordinationv1alpha1.HandoverPartitionOrderStatic:
			valuesMap := map[string]struct{}{}
			// create index of label values
			for _, v := range partitionLabelValues {
				valuesMap[v] = struct{}{}
			}
			// remove items with static order
			for _, v := range partition.Order.Static {
				delete(valuesMap, v)
			}
			// sort remaining items AlphaNumeric
			partitionLabelValues = make([]string, len(valuesMap))
			for v := range valuesMap {
				partitionLabelValues = append(partitionLabelValues, v)
			}
			sort.Strings(partitionLabelValues)
			// add static items in front and append sorted "other" values
			partitionLabelValues = append(partition.Order.Static, partitionLabelValues...)

		case coordinationv1alpha1.HandoverPartitionOrderAlphaNumeric:
			sort.Strings(partitionLabelValues)
		case coordinationv1alpha1.HandoverPartitionOrderNumeric:
			// TODO!
		}

		partitionGroups := groupByLabelValues(objects, partition.LabelKey, partitionLabelValues...)
		for i, pg := range partitionGroups {
			// split total into old and new
			groups := groupByLabelValues(
				pg, relabelSpec.LabelKey,
				relabelSpec.ToValue,
				relabelSpec.InitialValue,
			)
			newObjs := groups[0]
			oldObjs := groups[1]

			partitionStats = append(partitionStats, coordinationv1alpha1.HandoverPartitionStatus{
				Name:                 partitionLabelValues[i],
				HandoverCountsStatus: countObjectStatus(pg, probe, newObjs),
			})

			// fill processing queue with items from this partition.
			fillProcessingQueue(handover, oldObjs, relabelSpec.MaxUnavailable, unavailable)
		}
	}

	// fill processing queue with items from anywhere if no specific partition matches, or no partitions are set.
	fillProcessingQueue(handover, oldObjs, relabelSpec.MaxUnavailable, unavailable)

	// report stats
	handover.SetStats(totalStats, partitionStats)
	if totalStats.Found == totalStats.Updated {
		meta.SetStatusCondition(handover.GetConditions(), metav1.Condition{
			Type:               coordinationv1alpha1.HandoverCompleted,
			Status:             metav1.ConditionTrue,
			ObservedGeneration: handover.ClientObject().GetGeneration(),
			Reason:             "Complete",
			Message:            "All found objects have been re-labeled.",
		})
	} else {
		meta.SetStatusCondition(handover.GetConditions(), metav1.Condition{
			Type:               coordinationv1alpha1.HandoverCompleted,
			Status:             metav1.ConditionFalse,
			ObservedGeneration: handover.ClientObject().GetGeneration(),
			Reason:             "Incomplete",
			Message:            "Some found objects need to be re-labeled.",
		})
	}

	return ctrl.Result{}, nil
}

func (r *relabelReconciler) listAllObjects(
	ctx context.Context,
	handover genericHandover,
	objListType *unstructured.UnstructuredList, gvk schema.GroupVersionKind,
) ([]unstructured.Unstructured, error) {
	relabelSpec := handover.GetRelabelStrategy()

	// select all objects with new or old label value
	requirement, err := labels.NewRequirement(
		relabelSpec.LabelKey,
		selection.In,
		[]string{
			relabelSpec.ToValue,
			relabelSpec.InitialValue,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("building selector: %w", err)
	}
	selector := labels.NewSelector().Add(*requirement)

	if err := r.dynamicCache.List(
		ctx, objListType,
		client.InNamespace(handover.ClientObject().GetNamespace()),
		client.MatchingLabelsSelector{
			Selector: selector,
		},
	); err != nil {
		return nil, fmt.Errorf("listing %s: %w", gvk, err)
	}
	return objListType.Items, nil
}

func (r *relabelReconciler) handleProcessingSet(
	ctx context.Context,
	relabelSpec coordinationv1alpha1.HandoverStrategyRelabelSpec,
	objType *unstructured.Unstructured,
	probe probing.Prober,
	processing []coordinationv1alpha1.HandoverRefStatus,
) (stillProcessing []coordinationv1alpha1.HandoverRefStatus, err error) {
	for _, handoverRef := range processing {
		finished, err := r.processObject(
			ctx, relabelSpec, objType, probe, handoverRef)
		if err != nil {
			return stillProcessing, err
		}
		if !finished {
			stillProcessing = append(stillProcessing, handoverRef)
		}
	}
	return stillProcessing, nil
}

func (r *relabelReconciler) processObject(
	ctx context.Context,
	relabelSpec coordinationv1alpha1.HandoverStrategyRelabelSpec,
	objType *unstructured.Unstructured,
	probe probing.Prober,
	handoverRef coordinationv1alpha1.HandoverRefStatus,
) (finished bool, err error) {
	log := logr.FromContextOrDiscard(ctx)

	processingObj := objType.DeepCopy()
	err = r.dynamicCache.Get(ctx, client.ObjectKey{
		Name:      handoverRef.Name,
		Namespace: handoverRef.Namespace,
	}, processingObj)
	if errors.IsNotFound(err) {
		// Object gone, remove it from processing queue.
		finished = true
		err = nil
		return
	}
	if err != nil {
		return false, fmt.Errorf("getting object in process queue: %w", err)
	}

	// Relabel Strategy
	labels := processingObj.GetLabels()
	if labels == nil ||
		labels[relabelSpec.LabelKey] != relabelSpec.ToValue {
		labels[relabelSpec.LabelKey] = relabelSpec.ToValue
		processingObj.SetLabels(labels)
		if err := r.client.Update(ctx, processingObj); err != nil {
			return false, fmt.Errorf("updating object in process queue: %w", err)
		}
	}

	jsonPath := jsonpath.New("status-thing!!!").AllowMissingKeys(true)
	// TODO: SOOOO much validation for paths
	if err := jsonPath.Parse("{" + relabelSpec.ObservedLabelValuePath + "}"); err != nil {
		return false, fmt.Errorf("invalid jsonpath: %w", err)
	}

	statusValues, err := jsonPath.FindResults(processingObj.Object)
	if err != nil {
		return false, fmt.Errorf("getting status value: %w", err)
	}

	// TODO: even more proper handling
	if len(statusValues[0]) > 1 {
		return false, fmt.Errorf("multiple status values returned: %s", statusValues)
	}
	if len(statusValues[0]) == 0 {
		// no reported status
		return false, nil
	}

	statusValue := statusValues[0][0].Interface()
	if statusValue != relabelSpec.ToValue {
		log.Info("waiting for status field to update", "objName", handoverRef.Name)
		return false, nil
	}

	if success, message := probe.Probe(processingObj); !success {
		log.Info("waiting to be ready", "objName", handoverRef.Name, "failure", message)
		return false, nil
	}

	return true, nil
}

func fillProcessingQueue(
	handover genericHandover,
	objectsToHandover []unstructured.Unstructured,
	maxUnavailable, unavailable int,
) {
	processing := handover.GetProcessing()
	for _, obj := range objectsToHandover {
		if len(processing)+unavailable >= maxUnavailable {
			break
		}

		// add a new item to the processing queue
		processing = append(
			processing,
			coordinationv1alpha1.HandoverRefStatus{
				UID:       obj.GetUID(),
				Name:      obj.GetName(),
				Namespace: handover.ClientObject().GetNamespace(),
			})
	}
	handover.SetProcessing(processing)
}

func countObjectStatus(
	objects []unstructured.Unstructured,
	probe probing.Prober, newObjs []unstructured.Unstructured,
) coordinationv1alpha1.HandoverCountsStatus {
	// count total unavailable
	var unavailable int
	for _, obj := range objects {
		if success, _ := probe.Probe(&obj); !success {
			unavailable++
		}
	}

	// report counts
	found := int32(len(objects))
	return coordinationv1alpha1.HandoverCountsStatus{
		Found:     found,
		Updated:   int32(len(newObjs)),
		Available: found - int32(unavailable),
	}
}

func labelValues(objects []unstructured.Unstructured, labelKey string) []string {
	var out []string
	for _, obj := range objects {
		l := obj.GetLabels()
		if l != nil && len(l[labelKey]) > 0 {
			out = append(out, l[labelKey])
		}
	}
	return out
}

// given a list of objects this function will group all objects with the same label value.
// the return slice is guaranteed to be of the same size as the amount of values given to the function.
func groupByLabelValues(in []unstructured.Unstructured, labelKey string, values ...string) [][]unstructured.Unstructured {
	out := make([][]unstructured.Unstructured, len(values))
	for _, obj := range in {
		if obj.GetLabels() == nil {
			continue
		}
		if len(obj.GetLabels()[labelKey]) == 0 {
			continue
		}

		for i, v := range values {
			if obj.GetLabels()[labelKey] == v {
				out[i] = append(out[i], obj)
			}
		}
	}
	return out
}
