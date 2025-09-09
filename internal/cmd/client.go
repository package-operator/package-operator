package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"

	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
	manifestsv1alpha1 "package-operator.run/apis/manifests/v1alpha1"
)

func NewClient(client client.Client) *Client {
	return &Client{
		client: client,
	}
}

type Client struct {
	client client.Client
}

func (c *Client) GetObjectset(
	ctx context.Context,
	packageName string,
	packageNamespace string,
) (*corev1alpha1.ObjectSet, error) {
	objreslist := &corev1alpha1.ObjectSetList{}
	if err := c.client.List(ctx, objreslist, client.InNamespace(packageNamespace)); err != nil {
		return nil, fmt.Errorf("getting package objectsetlist : %w", err)
	}
	for _, objectSet := range objreslist.Items {
		if meta.IsStatusConditionTrue(objectSet.Status.Conditions, corev1alpha1.ObjectSetAvailable) &&
			strings.Contains(objectSet.Name, packageName) {
			return &objectSet, nil
		}
	}
	return nil, errors.New("ObjectSet could not be found")
}

func (c *Client) GetClusterObjectset(
	ctx context.Context,
	packageName string,
) (*corev1alpha1.ClusterObjectSet, error) {
	clusterObjectSetList := &corev1alpha1.ClusterObjectSetList{}
	if err := c.client.List(ctx, clusterObjectSetList); err != nil {
		return nil, fmt.Errorf("getting package objectsetlist : %w", err)
	}
	for _, clusterObjectSet := range clusterObjectSetList.Items {
		if meta.IsStatusConditionTrue(clusterObjectSet.Status.Conditions, corev1alpha1.ObjectSetAvailable) &&
			strings.Contains(clusterObjectSet.Name, packageName) {
			return &clusterObjectSet, nil
		}
	}
	return nil, errors.New("ClusterObjectSet could not be found")
}

func (c *Client) GetPackage(ctx context.Context, name string, opts ...GetPackageOption) (*Package, error) {
	var cfg GetPackageConfig

	cfg.Option(opts...)

	var obj client.Object

	if cfg.Namespace != "" {
		obj = &corev1alpha1.Package{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: cfg.Namespace,
			},
		}
	} else {
		obj = &corev1alpha1.ClusterPackage{
			ObjectMeta: metav1.ObjectMeta{
				Name: name,
			},
		}
	}

	if err := c.client.Get(ctx, client.ObjectKeyFromObject(obj), obj); err != nil {
		return nil, fmt.Errorf("getting package object: %w", err)
	}

	return &Package{
		client: c.client,
		obj:    obj,
	}, nil
}

type GetPackageConfig struct {
	Namespace string
}

func (c *GetPackageConfig) Option(opts ...GetPackageOption) {
	for _, opt := range opts {
		opt.ConfigureGetPackage(c)
	}
}

type GetPackageOption interface{ ConfigureGetPackage(*GetPackageConfig) }

func (c *Client) GetObjectDeployment(
	ctx context.Context, name string, opts ...GetObjectDeploymentOption,
) (*ObjectDeployment, error) {
	var cfg GetObjectDeploymentConfig

	cfg.Option(opts...)

	var obj client.Object

	if cfg.Namespace != "" {
		obj = &corev1alpha1.ObjectDeployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: cfg.Namespace,
			},
		}
	} else {
		obj = &corev1alpha1.ClusterObjectDeployment{
			ObjectMeta: metav1.ObjectMeta{
				Name: name,
			},
		}
	}

	if err := c.client.Get(ctx, client.ObjectKeyFromObject(obj), obj); err != nil {
		return nil, fmt.Errorf("getting objectdeployment object: %w", err)
	}

	return &ObjectDeployment{
		client: c.client,
		obj:    obj,
	}, nil
}

type GetObjectDeploymentConfig struct {
	Namespace string
}

func (c *GetObjectDeploymentConfig) Option(opts ...GetObjectDeploymentOption) {
	for _, opt := range opts {
		opt.ConfigureGetObjectDeployment(c)
	}
}

type GetObjectDeploymentOption interface {
	ConfigureGetObjectDeployment(*GetObjectDeploymentConfig)
}

type Package struct {
	client client.Client
	obj    client.Object
}

func (p *Package) Name() string {
	return p.obj.GetName()
}

func (p *Package) Namespace() string {
	return p.obj.GetNamespace()
}

func (p *Package) CurrentRevision() int64 {
	if cpkg, ok := p.obj.(*corev1alpha1.ClusterPackage); ok {
		return cpkg.Status.Revision
	}

	return p.obj.(*corev1alpha1.Package).Status.Revision
}

func (p *Package) ObjectSets(ctx context.Context) (ObjectSetList, error) {
	opts := []findObjectSetsOption{
		withSelector{
			Selector: labels.SelectorFromSet(labels.Set{
				manifestsv1alpha1.PackageInstanceLabel: p.Name(),
			}),
		},
	}

	if _, ok := p.obj.(*corev1alpha1.Package); ok {
		opts = append(opts, withNamespace(p.Namespace()))
	}

	return findObjectSets(ctx, p.client, opts...)
}

type ObjectDeployment struct {
	client client.Client
	obj    client.Object
}

func (d *ObjectDeployment) Name() string {
	return d.obj.GetName()
}

func (d *ObjectDeployment) Namespace() string {
	return d.obj.GetNamespace()
}

func (d *ObjectDeployment) CurrentRevision() int64 {
	if cod, ok := d.obj.(*corev1alpha1.ClusterObjectDeployment); ok {
		return cod.Status.Revision
	}

	return d.obj.(*corev1alpha1.ObjectDeployment).Status.Revision
}

func (d *ObjectDeployment) ObjectSets(ctx context.Context) (ObjectSetList, error) {
	opts := []findObjectSetsOption{
		withSelector{
			Selector: labels.SelectorFromSet(d.obj.GetLabels()),
		},
	}

	if _, ok := d.obj.(*corev1alpha1.ObjectDeployment); ok {
		opts = append(opts, withNamespace(d.Namespace()))
	}

	return findObjectSets(ctx, d.client, opts...)
}

func findObjectSets(ctx context.Context, c client.Client, opts ...findObjectSetsOption) (ObjectSetList, error) {
	var cfg findObjectSetsConfig

	cfg.Option(opts...)

	listOpts := []client.ListOption{
		client.MatchingLabelsSelector{
			Selector: cfg.Selector,
		},
	}

	if cfg.Namespace != "" {
		listOpts = append(listOpts, client.InNamespace(cfg.Namespace))

		var sets corev1alpha1.ObjectSetList

		if err := c.List(ctx, &sets, listOpts...); err != nil {
			return nil, fmt.Errorf("listing ObjectSets: %w", err)
		}

		revs := make(ObjectSetList, 0, len(sets.Items))
		for i := range sets.Items {
			revs = append(revs, NewObjectSet(&sets.Items[i]))
		}

		return revs, nil
	}

	var sets corev1alpha1.ClusterObjectSetList

	if err := c.List(ctx, &sets, listOpts...); err != nil {
		return nil, fmt.Errorf("listing ClusterObjectSets: %w", err)
	}

	revs := make(ObjectSetList, 0, len(sets.Items))
	for i := range sets.Items {
		revs = append(revs, NewObjectSet(&sets.Items[i]))
	}

	return revs, nil
}

type findObjectSetsConfig struct {
	Namespace string
	Selector  labels.Selector
}

func (c *findObjectSetsConfig) Option(opts ...findObjectSetsOption) {
	for _, opt := range opts {
		opt.ConfigureFindObjectSets(c)
	}
}

type findObjectSetsOption interface {
	ConfigureFindObjectSets(*findObjectSetsConfig)
}

type withNamespace string

func (w withNamespace) ConfigureFindObjectSets(c *findObjectSetsConfig) {
	c.Namespace = string(w)
}

type withSelector struct{ Selector labels.Selector }

func (w withSelector) ConfigureFindObjectSets(c *findObjectSetsConfig) {
	c.Selector = w.Selector
}

func NewObjectSet(obj client.Object) ObjectSet {
	return ObjectSet{
		obj: obj,
	}
}

type ObjectSet struct {
	obj client.Object
}

func (s *ObjectSet) Name() string {
	return s.obj.GetName()
}

func (s *ObjectSet) Namespace() string {
	return s.obj.GetNamespace()
}

func (s *ObjectSet) HasSucceeded() bool {
	return meta.IsStatusConditionTrue(s.GetSpecConditions(), corev1alpha1.ObjectSetSucceeded)
}

func (s *ObjectSet) GetSpecConditions() []metav1.Condition {
	if cos, ok := s.obj.(*corev1alpha1.ClusterObjectSet); ok {
		return cos.Status.Conditions
	}

	return s.obj.(*corev1alpha1.ObjectSet).Status.Conditions
}

func (s *ObjectSet) Revision() int64 {
	if cos, ok := s.obj.(*corev1alpha1.ClusterObjectSet); ok {
		return cos.Spec.Revision
	}

	return s.obj.(*corev1alpha1.ObjectSet).Spec.Revision
}

func (s *ObjectSet) ChangeCause() string {
	const changeCauseKey = "kubernetes.io/change-cause"

	return s.obj.GetAnnotations()[changeCauseKey]
}

func (s *ObjectSet) MarshalYAML() ([]byte, error) {
	return yaml.Marshal(s.obj)
}

func (s *ObjectSet) MarshalJSON() ([]byte, error) {
	return json.Marshal(s.obj)
}

type ObjectSetList []ObjectSet

func (l ObjectSetList) Sort() {
	slices.SortFunc(l, func(a, b ObjectSet) int {
		return int(a.Revision() - b.Revision())
	})
}

func (l ObjectSetList) FindRevision(rev int64) (ObjectSet, bool) {
	idx := slices.IndexFunc(l, func(os ObjectSet) bool {
		return os.Revision() == rev
	})
	if idx < 0 {
		return ObjectSet{}, false
	}

	return l[idx], true
}

func (l ObjectSetList) RenderJSON() ([]byte, error) {
	return json.MarshalIndent(l, "", "    ")
}

func (l ObjectSetList) RenderYAML() ([]byte, error) {
	return yaml.Marshal(l)
}

func (l ObjectSetList) RenderTable(headers ...string) Table {
	table := NewDefaultTable(
		WithHeaders(headers),
	)

	for _, rev := range l {
		table.AddRow(
			Field{
				Name:  "Revision",
				Value: rev.Revision(),
			},
			Field{
				Name:  "Successful",
				Value: rev.HasSucceeded(),
			},
			Field{
				Name:  "Change-Cause",
				Value: rev.ChangeCause(),
			},
		)
	}

	return table
}
