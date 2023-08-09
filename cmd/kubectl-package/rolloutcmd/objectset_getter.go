package rolloutcmd

import (
	"context"
	"errors"
	"fmt"
	"strings"

	internalcmd "package-operator.run/internal/cmd"
)

func newObjectSetGetter(client *internalcmd.Client) *objectSetGetter {
	return &objectSetGetter{
		client: client,
	}
}

type objectSetGetter struct {
	client *internalcmd.Client
}

func (g *objectSetGetter) GetObjectSets(ctx context.Context, rsrc, name, ns string) (internalcmd.ObjectSetList, error) {
	var err error
	var getter interface {
		ObjectSets(context.Context) (internalcmd.ObjectSetList, error)
	}

	switch strings.ToLower(rsrc) {
	case "clusterpackage":
		getter, err = g.client.GetPackage(ctx, name)
	case "package":
		getter, err = g.client.GetPackage(ctx, name, internalcmd.WithNamespace(ns))
	case "clusterobjectdeployment":
		getter, err = g.client.GetObjectDeployment(ctx, name)
	case "objectdeployment":
		getter, err = g.client.GetObjectDeployment(ctx, name, internalcmd.WithNamespace(ns))
	default:
		return nil, errInvalidResourceType
	}

	if err != nil {
		return nil, fmt.Errorf("getting resource %s/%s: %w", rsrc, name, err)
	}

	return getter.ObjectSets(ctx)
}

var errInvalidResourceType = errors.New("invalid resource type")
