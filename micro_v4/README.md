# micro v4

## Usage

```golang
import(
    sm "github.com/sentinel-group/sentinel-go-adapters/micro/v4"
)

srv := micro.NewService(
    micro.WrapHandler(sm.NewHandlerWrapper()),
)
```
