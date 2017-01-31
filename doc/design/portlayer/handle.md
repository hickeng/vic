# Handle specification

Use of a handle should be _extremely_ simple - it is provided as the result of a portlayer call, and can be passed back into portlayer calls.
The handle is opaque to the caller meaning that the format, content, and structure should not be assumed or manipulated outside of the portlayer calls, even
if those facets are known.

The handle provides a mechanism of passing a configuration between components without each component needing to share state with the others, or to know details
about how other components perform configuration.


## Handle use in the portlayer

When an element is joined to a handle it should add characteristics it provides and those it requires. Those characteristics should remain associated with
that specific element so that:
* unbind/leave calls can operate cleanly
* unresolved requirements can be tied back to specific elements

```go
type Handle interface {
    // The rolled up set of characteristics from each of the joined elements
    // TODO: should Bind be able to add more capabilities?
    //
    // e.g. system.loader.abi=linux
    //      system.loader.version=4.0
    //      system.executor.arch=x86_64
    //      system.executor.parent
    Provides() map[string][string]

    // The rolled up set of requirements from each of the joined elements
    // 
    Requires() map[string][string]
}




