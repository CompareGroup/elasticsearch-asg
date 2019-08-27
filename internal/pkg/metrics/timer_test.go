package metrics

import (
	"errors"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/mock"

	"github.com/mintel/elasticsearch-asg/internal/pkg/metrics/mocks"
)

func TestVecTimer_ObserveErr(t *testing.T) {
	vec := new(mocks.ObserverVec)
	o := new(mocks.Observer)
	vec.On("With", prometheus.Labels{LabelStatus: "error"}).Return(o)
	o.On("Observe", mock.AnythingOfType("float64")).Return()

	fakeErr := errors.New("bad things!")

	func() {
		var err error
		timer := NewVecTimer(vec)
		defer func() { timer.ObserveErr(err) }()

		err = fakeErr
	}()

	vec.AssertExpectations(t)
	o.AssertExpectations(t)
}
