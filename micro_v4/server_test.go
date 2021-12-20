package micro

import (
	"context"
	"errors"
	"log"
	"testing"
	"time"

	sentinel "github.com/alibaba/sentinel-golang/api"
	"github.com/alibaba/sentinel-golang/core/base"
	"github.com/alibaba/sentinel-golang/core/flow"
	"github.com/alibaba/sentinel-golang/core/stat"
	"github.com/alibaba/sentinel-golang/util"
	"github.com/stretchr/testify/assert"
	"go-micro.dev/v4"
	"go-micro.dev/v4/server"

	proto "github.com/sentinel-group/sentinel-go-adapters/micro/v4/test"
)

const FakeErrorMsg = "fake error for testing"

type TestHandler struct{}

func (h *TestHandler) Ping(ctx context.Context, req *proto.Request, rsp *proto.Response) error {
	rsp.Result = "Pong"
	return nil
}

func TestServerLimiter(t *testing.T) {
	svr := micro.NewService(
		micro.Address("localhost:56436"),
		micro.Name("sentinel.test.server"),
		micro.Version("latest"),
		micro.WrapHandler(NewHandlerWrapper(
			// add custom fallback function to return a fake error for assertion
			WithServerBlockFallback(
				func(ctx context.Context, request server.Request, blockError *base.BlockError) error {
					return errors.New(FakeErrorMsg)
				}),
		)),
	)

	_ = proto.RegisterTestHandler(svr.Server(), &TestHandler{})

	go svr.Run()

	time.Sleep(time.Second)

	c := svr.Client()
	req := c.NewRequest("sentinel.test.server", "Test.Ping", &proto.Request{})

	err := sentinel.InitDefault()
	if err != nil {
		log.Fatal(err)
	}

	_, err = flow.LoadRules([]*flow.Rule{
		{
			Resource:               req.Method(),
			Threshold:              1.0,
			TokenCalculateStrategy: flow.Direct,
			ControlBehavior:        flow.Reject,
		},
	})

	assert.Nil(t, err)

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
		assert.Nil(t, err)
		assert.EqualValues(t, "Pong", rsp.Result)
		assert.True(t, util.Float64Equals(1.0, stat.GetResourceNode(req.Method()).GetQPS(base.MetricEventPass)))

		t.Run("second fail", func(t *testing.T) {
			err := c.Call(context.TODO(), req, rsp)
			assert.Error(t, err)
			assert.True(t, util.Float64Equals(1.0, stat.GetResourceNode(req.Method()).GetQPS(base.MetricEventPass)))
		})
	})
}
