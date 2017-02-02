# Candidate components

The following are candidates for components. A component:
* has knowledge of how to provide a specific resource
* manages distinct elements of that resource


Questions:
* task cancellation/addressing CDE

* how is power on/off done?
* how is suspend resume done?
  * Join a hibernation mechanism to add capability. Bind/Unbind to suspend/resume.
* can the API style phrase services?
* can the API style handle pass-through devices, e.g. GPGPU
* how are affinity constraints phrased? do they need to be present at creation time or usage time?
* are scopes and stores the same conceptual construct?


Facets required to start a container:
* Execution environment - slicable basic compute capabilities
  * ESX, vCenter, Linux, Windows
* Executor - sliced portion of environment - isolation mechanism
  * VM, process, namespace/cgroup
* Loader - mechanism for initialization of an Executor, if needed
  * bootstrap.iso, direct boot, customer-bootstrap.iso
* Linker - provides access to data sources/sinks
  * volume mounts, networks, etc
* Logic - the environment required by a task
  * filesystem, executable
* Task - invocation of a Logic
  * working dir, env vars, process arguments, credentials, 



Current:
logic - storage.Join
task - exec.Join
loader - not selectable
executor - not selectable, can configure memory/cpu
linker - volume.Join/network.Join/logging.Join/interaction.Join
execution env - exec.ChangeState


# Policy

We may need to be able to supply policy at creation time, composition, or usage time.

Questions:
* how, exactly, are NIOC & SIOC specified
* should policy be associated with scopes/stores?
* should policy be associated with the memebership of a scope/store? i.e. per-element during Join
  * policy could be the pairing of a filterspec with a constraint (read-only, anti-afinity, allowed, denied, etc)
    - if the constraint is unknown, cannot be satisfied or interpreted given existing policy then it's an error
    - filter spec could be used to specify outgoing target in the case of open network scopes

Scopes/Stores allow:
* create, write
* priority/shares
* affinity

Exposed ports:
* may be associated with a specific scope
  * if a contianer is in a scope then it may connect to any exposed ports in the scope

Initiating connections:
* only a concern on open scopes (bridge, container-networks)
  * container may define permitted outgoing target (tcp: address/port, ip:address, ether:mac, etc)

# Component interface

```go
type Component interface {
    // Is the filterspec a per component "language" or something common
    // between them?
    //
    // Should this allow for general "uses X" queries? When trying to delete an image for example
    // the storage component isn't necessary going to be able to tell that a disk is in use by container X.
    // If we can query exec for "uses image I" then the caller can determine what is blocking deletion.
    //
    // Default filterspec should hide _infrastructure_ elements (system.hidden, system.infrastructure tags? system.type=hidden?)
    List(filterspec interface{})

    // Unsure about this one - while there needs to be a way to provide mocked configuration to the components
    // is that best left as an implementation detail rather than specified here?
    Configure(config interface{})

    Events

    Diagnostics
}

type Events interface {
    // Is this filterspec the same "language" as for List? (presuming common language between components)
    Events(filterspec interface{}) <-chan Event

    // No idea whether there should be separate calls for async vs sync, inline trigger, allowing the aspect
    // to mutate state, etc.
    Trigger(filterspec interface{}, aspect(Event))
}

type Diagnostics {
    // Operations that are currently in progress
    //   - not sure if this is worth the data management overhead - implies need for collation and tracking
    InProgress

    // Provides details about performance statics
    // 
    // This could be a separate interface
    Statistics(filterspec ???)

    // Query errors
    //  * time range
    //  * error type
    //  * target elements
    //  * ...
    Errors(filterspec ???)

    // Return logs
    // * time range
    // * operation ID
    // * errors
    // - or just text files and size limit?
    Log(filterspec ???)
}
```


# Composition interface

```go
  type Compose {
    // Adds a capability.
    //
    // Adds to the handle to allow for later activation. If different variants are needed
    // for different operating systems or environments, then this should supply all off the variations
    // tagged such that a singluar variant can be chosen by Commit.
    Join(handle Handle, id ID, ...)

    // Activates a capability. If id is omitted, activiates all elements relating
    // to this component.
    // Arguments supply details as to _how_ this capability presents
    Bind(handle Handle, id ID, ...)

    // Deactivates a capability.
    Unbind(handle Handle, id ID)

    // Removes a capability
    Leave(handle Handle, id ID)
  }
```

# Commit

The entire port layer should be as close to idempotent as viable, with exceptions _clearly_ documented. The downside of that is that people commonly use success or failure of racing operations to determine where follow on work happens and an idempotent model can invalidate the underlying assumptions of that approach.

Example - power on.

Actor1: power on X  # succeeds
Actor2: power on X  # succeeds in idempotent case, fails otherwise

As such we should consider identifying which portions of a change actually altered state:

```go
type CommitStatus int8
const (
    // Successfully applied change
    COMMIT_SUCCESS CommitStatus = iota
    // Failed to apply change
    COMMIT_ERROR
    // Change is a noop
    COMMIT_NOOP
)

type Result interface {
    // were there any errors in the result
    IsError()   bool
    // result per element
    Results()  map[ID]CommitStatus
    // detail messages
    Detail()   map[ID]string
}

type interface Commit {
    Commit (handle Handle) Result
}
```


# Errors

The following are some general errors that could occur:

vmomi errors (should be moved to vmomi gateway, but needs to be such that can be passed through the components):
 * connection failure
 * authentication failure
 * authorization error
 * vsphere system error

runtime errors:
 * tether should report status of element initialization
 * failure messages should be actionable, or informative
 * error status for an element should be such that it can be mapped back to the element ID
