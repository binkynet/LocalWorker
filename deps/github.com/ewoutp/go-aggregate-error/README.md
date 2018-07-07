# AggregateError

AggregateError is an error structure used to aggregate multiple errors in a thread safe way.

# Adding errors

```
var ae AggregateError
ae.Add(err)
```

# Have errors been added?

```
var ae AggregateError
...
if ae.IsEmpty() {
    fmt.Println("No errors found")
}
```
