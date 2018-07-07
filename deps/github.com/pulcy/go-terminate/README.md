# go-terminate

[![GoDoc](https://godoc.org/github.com/pulcy/go-terminate?status.svg)](http://godoc.org/github.com/pulcy/go-terminate)

Utility class used to terminate go services in a controlled manor.

## Usage

```
import (
    "log"

    "github.com/pulcy/go-terminate"
    )

func main() {
    t := terminate.NewTerminator(log.Printf, nil)
    go t.ListenSignals()
    ...
}
```
