package micro

import (
	"context"
	"errors"
	"log"
	"testing"

	sentinel "github.com/alibaba/sentinel-golang/api"
	"github.com/alibaba/sentinel-golang/core/base"
	"github.com/alibaba/sentinel-golang/core/flow"
	"github.com/alibaba/sentinel-golang/core/stat"
	"github.com/alibaba/sentinel-golang/util"
	"github.com/asim/go-micro/plugins/registry/memory/v4"
	proto "github.com/sentinel-group/sentinel-go-adapters/micro/v4/test"
	"github.com/stretchr/testify/assert"
	"go-micro.dev/v4/client"
	"go-micro.dev/v4/selector"
)

func TestClientLimiter(t *testing.T) {
	// setup
	r := memory.NewRegistry()
	s := selector.NewSelector(selector.Registry(r))

	c := client.NewClient(
		// set the selector
		client.Selector(s),
		// add the breaker wrapper
		client.Wrap(NewClientWrapper(
			// add custom fallback function to return a fake error for assertion
			WithClientBlockFallback(
				func(ctx context.Context, request client.Request, blockError *base.BlockError) error {
					return errors.New(FakeErrorMsg)
				}),
		)),
	)

	req := c.NewRequest("sentinel.test.server", "Test.Ping", &proto.Request{}, client.WithContentType("application/json"))

	err := sentinel.InitDefault()
	if err != nil {
		log.Fatal(err)
	}

	rsp := &proto.Response{}

	t.Run("success", func(t *testing.T) {
		_, err := flow.LoadRules([]*flow.Rule{
			{
				Resource:               req.Method(),
				Threshold:              1.0,
				TokenCalculateStrategy: flow.Direct,
				ControlBehavior:        flow.Reject,
			},
		})
		assert.Nil(t, err)
		err = c.Call(context.TODO(), req, rsp)
		// No server started, the return err should not be nil
		assert.NotNil(t, err)
		assert.NotEqual(t, FakeErrorMsg, err.Error())
		assert.True(t, util.Float64Equals(1.0, stat.GetResourceNode(req.Method()).GetQPS(base.MetricEventPass)))

		t.Run("second fail", func(t *testing.T) {
			err := c.Call(context.TODO(), req, rsp)
			assert.EqualError(t, err, FakeErrorMsg)
			assert.True(t, util.Float64Equals(1.0, stat.GetResourceNode(req.Method()).GetQPS(base.MetricEventPass)))
		})
	})
}