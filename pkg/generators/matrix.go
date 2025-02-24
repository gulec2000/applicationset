package generators

import (
	"errors"
	"fmt"
	"time"

	argoprojiov1alpha1 "github.com/argoproj/applicationset/api/v1alpha1"
	"github.com/argoproj/applicationset/pkg/utils"
)

var _ Generator = (*MatrixGenerator)(nil)

var (
	ErrMoreThanTwoGenerators      = errors.New("found more than two generators, Matrix support only two")
	ErrLessThanTwoGenerators      = errors.New("found less than two generators, Matrix support only two")
	ErrMoreThenOneInnerGenerators = errors.New("found more than one generator in matrix.Generators")
)

type MatrixGenerator struct {
	// The inner generators supported by the matrix generator (cluster, git, list...)
	supportedGenerators map[string]Generator
}

func NewMatrixGenerator(supportedGenerators map[string]Generator) Generator {
	m := &MatrixGenerator{
		supportedGenerators: supportedGenerators,
	}
	return m
}

func (m *MatrixGenerator) GenerateParams(appSetGenerator *argoprojiov1alpha1.ApplicationSetGenerator, appSet *argoprojiov1alpha1.ApplicationSet) ([]map[string]string, error) {

	if appSetGenerator.Matrix == nil {
		return nil, EmptyAppSetGeneratorError
	}

	if len(appSetGenerator.Matrix.Generators) < 2 {
		return nil, ErrLessThanTwoGenerators
	}

	if len(appSetGenerator.Matrix.Generators) > 2 {
		return nil, ErrMoreThanTwoGenerators
	}

	res := []map[string]string{}

	g0, err := m.getParams(appSetGenerator.Matrix.Generators[0], appSet)
	if err != nil {
		return nil, err
	}
	g1, err := m.getParams(appSetGenerator.Matrix.Generators[1], appSet)
	if err != nil {
		return nil, err
	}

	for _, a := range g0 {
		for _, b := range g1 {
			val, err := utils.CombineStringMaps(a, b)
			if err != nil {
				return nil, err
			}
			res = append(res, val)
		}
	}

	return res, nil
}

func (m *MatrixGenerator) getParams(appSetBaseGenerator argoprojiov1alpha1.ApplicationSetNestedGenerator, appSet *argoprojiov1alpha1.ApplicationSet) ([]map[string]string, error) {
	var matrix *argoprojiov1alpha1.MatrixGenerator
	if appSetBaseGenerator.Matrix != nil {
		// Since nested matrix generator is represented as a JSON object in the CRD, we unmarshall it back to a Go struct here.
		nestedMatrix, err := argoprojiov1alpha1.ToNestedMatrixGenerator(appSetBaseGenerator.Matrix)
		if err != nil {
			return nil, fmt.Errorf("unable to unmarshall nested matrix generator: %v", err)
		}
		if nestedMatrix != nil {
			matrix = nestedMatrix.ToMatrixGenerator()
		}
	}

	var mergeGenerator *argoprojiov1alpha1.MergeGenerator
	if appSetBaseGenerator.Merge != nil {
		// Since nested merge generator is represented as a JSON object in the CRD, we unmarshall it back to a Go struct here.
		nestedMerge, err := argoprojiov1alpha1.ToNestedMergeGenerator(appSetBaseGenerator.Merge)
		if err != nil {
			return nil, fmt.Errorf("unable to unmarshall nested merge generator: %v", err)
		}
		if nestedMerge != nil {
			mergeGenerator = nestedMerge.ToMergeGenerator()
		}
	}

	t, err := Transform(
		argoprojiov1alpha1.ApplicationSetGenerator{
			List:                    appSetBaseGenerator.List,
			Clusters:                appSetBaseGenerator.Clusters,
			Git:                     appSetBaseGenerator.Git,
			SCMProvider:             appSetBaseGenerator.SCMProvider,
			ClusterDecisionResource: appSetBaseGenerator.ClusterDecisionResource,
			PullRequest:             appSetBaseGenerator.PullRequest,
			Matrix:                  matrix,
			Merge:                   mergeGenerator,
		},
		m.supportedGenerators,
		argoprojiov1alpha1.ApplicationSetTemplate{},
		appSet)

	if err != nil {
		return nil, fmt.Errorf("child generator returned an error on parameter generation: %v", err)
	}

	if len(t) == 0 {
		return nil, fmt.Errorf("child generator generated no parameters")
	}

	if len(t) > 1 {
		return nil, ErrMoreThenOneInnerGenerators
	}

	return t[0].Params, nil
}

const maxDuration time.Duration = 1<<63 - 1

func (m *MatrixGenerator) GetRequeueAfter(appSetGenerator *argoprojiov1alpha1.ApplicationSetGenerator) time.Duration {
	res := maxDuration
	var found bool

	for _, r := range appSetGenerator.Matrix.Generators {
		base := &argoprojiov1alpha1.ApplicationSetGenerator{
			List:     r.List,
			Clusters: r.Clusters,
			Git:      r.Git,
		}
		generators := GetRelevantGenerators(base, m.supportedGenerators)

		for _, g := range generators {
			temp := g.GetRequeueAfter(base)
			if temp < res && temp != NoRequeueAfter {
				found = true
				res = temp
			}
		}
	}

	if found {
		return res
	} else {
		return NoRequeueAfter
	}

}

func (m *MatrixGenerator) GetTemplate(appSetGenerator *argoprojiov1alpha1.ApplicationSetGenerator) *argoprojiov1alpha1.ApplicationSetTemplate {
	return &appSetGenerator.Matrix.Template
}
