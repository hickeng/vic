# Task

The task component deals with specification of tasks to run within a container, and encompasses items such as:
* log destination
* environment
* credentials
* command and arguments

It could also encompass task level policy such as:
* interaction policy
* quotas, limits, & deadlines
* restart behaviour

## Task - configuration

Tasks can be associated with storage (images or volumes), or with executors (containers).

This implies that tasks associated with storage should be defined with absolute paths, but that they should be interpreted relative to the mount point of the storage element when invoked.


## Task - element management

```go
// semi structured identifier
// Includes
//  * location - e.g. volume store or image store
//  * ID - identifier within it's store
typedef ID url.URL

// Descripor for a storage element
// should implement interface(s) required for filtering
type Storage struct {
    id          ID
    parentID    ID                      // optional
    roBlob      map[string][]byte
    rwBlob      map[string][]byte
    data        io.ReadCloser           // optional
    checksum    string                  // optional? if we require this it could help with full stack TPM 
    auditLog    ???
}


type Task interface {
    // Creates a new task element
    //  * ID - encodes both location/store in which to create it and the name by which it's addressed
    //  * parentID - 
    //  * roUser - caller specific metadata that is immutable once created
    //  * rwUser - caller specific metadata that can be updated - keys must not exist in the roUser set
    //
    Create(id ID, parentID ID, data io.ReadCloser, checksum string, , roUser map[string][]byte, rwUser map[string][]byte) error

    // Should provide options to control whether this returns:
    //  * rwUser/roUser/attributes
    //         - should allow caller to specify only portions of metadata to return
    //         - should return all user data and attributes by default
    Read(id ID, filterspec ???) (Storage, error)

    // Updates the various modifiable aspects of the element
    //  * ID must exist
    //  * can only be updated if location policy allows writing - applies to rwUser data
    //  * rwUser data - if the value of a key is nil it will be deleted from the store
    Update(id ID, rwUser map[string][]byte) error

    // Deletes all aspects of this element, including the implementation assocaited metadata and user ro/rw metadata
    Delete(id ID) error
}
```

## Task - element management errors

**Create** - ID
 * unknown location
 * invalid location - e.g. read-only
 * name collision

**Create** - ParentID
 * unknown parent - id not found
 * invalid parent - provided ID cannot be used as parent

**Create** - roUser & rwUser
 * invalid entry - does not allow nil values, or key (or value) violates kv store constraints
 * runtime error - unable to persist for whatever reason
 * read only key - if a key exists in the roUser data then it must not be in the rwUser data


**Read** - ID
 * unknown location
 * id not found in location

**Read** - filterspec
 * invalid spec



**Update** - ID
 * unknown location
 * id not found in location
 * invalid location - e.g. read-only or create only

**Update** - roUser & rwUser
 * invalid entry - does not allow nil values, or key (or value) violates kv store constraints
 * runtime error - unable to persist for whatever reason
 * read only key - if a key exists in the roUser data then it must not be in the rwUser data


**Delete** - ID
 * unknown location
 * id not found in location
 * invalid location - e.g. read-only or create only

**Delete** - Invalid states
 * in use - could provide list of referencing IDs (guess this would be storage IDs only)
 * permission denied (if we have sub-location permissions)


## Task - events

Possible event set:
* created
* modified
* deleted
* bound - started
* unbound - stopped


## Task - composition

Questions:
* could Bind be used for process interaction?
* how does signalling work - we could provide a separate _Interrupt_ interface
* how is logging configured?
  - Join various logging mechanisms to the executor to supply base support
  - bind to activate mechanism for Tasks that have required it
  - how does a Task identify which logging mechansim is required?
  - is it one logging mechanism per executor and a Task only gets to chose if logging occurs?

```go
typedef interface{} Handle
type Composition interface {
    // handle - this is the opaque handle that represents a current composition

    // id - the element to add to this composition
    Join(handle Handle, id ID, ...?) (Handle, error)

    // id - the specific element to do second stage processing on. Optional.
    //    - if not specified then all elements managed by the component will be processed
    Bind(handle Handle, id ID, ...?) (Handle, error)

    // id - the specific element to do undo second stage processing on. Optional.
    //    - if not specified then all elements managed by the component will be processed
    Unbind(handle Handle, id ID) (Handle, error)

    // id - the element to remove from this composition
    Leave(handle Handle, id ID) (Handle, error)
}
```

### Composition errors


### Persistence

Tasks should be persisted in:
* storage metadata (if the storage element has had a task associated with it)
* executor metadata

## Questions and acceptance criteria:

* Example workflow for creating image with scratch parent
* Example workflow for creating image with specific parent
* Example workflow for creating metadata only image - is this actually desired? Do we want to support non-tagged layers?

* Example workflow for creating fresh volume
* Example workflow for creating volume from parent
* Example workflow for creating nfs volume
* Example workflow for creating nfs volume from parent (error case)

* Example workflow for use of image
* Example workflow for use of volume and image
* Example workflow for non-persistent volume
* Example workflow for read-only root filesystem

* Example workflow for listing specific images

* Example workflow for creating image with freespace requirement requiring resize

* What happens if no storage is mapped to / in a container?