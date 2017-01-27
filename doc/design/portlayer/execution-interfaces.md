# Storage

The storage component handles both volumes and images, using the same API. We do not distinguish between usage in that fashion.

## Storage - configuration

### Locations

These are storage locations such as the volume stores and image stores that we currently provide. These locations provide the following:
* organisation mechanism
* policy application
  * IO performance
  * Permissions (create, read, write)
  * Encryption


## Storage - element management

```go
// semi structured identifier
// Includes
//  * location - e.g. volume store or image store
//  * ID - identifier within it's store
typedef ID url.URL

// Descripor for an execution element
// should implement interface(s) required for filtering
type Executor struct {
    id          ID
    parentID    ID
    cpu         MHz                     // should this be cores or MHz
    mem         int64                   // MB
    restart     interface{}             // policy? restart count? error types suppressing restart?
    roBlob      map[string][]byte
    rwBlob      map[string][]byte
    auditLog    ???
}


type Executor interface {
    // Creates a new storage element
    //  * ID - encodes both location/store in which to create it and the name by which it's addressed
    //  * parentID - the parent to inherit from. May be nil. The implementation will generate an error if the parent is not viable
    //  * roUser - caller specific metadata that is immutable once created
    //  * rwUser - caller specific metadata that can be updated - keys must not exist in the roUser set
    //
    //  Should this return Executor or just an error?
    //
    // executor specific:
    // * bootstrap mechanism - is this just a preconfigured Executor "parent"?
    // * cpu & mem
    //
    // dispatch specific:
    // * env vars
    // * working directory
    // * argv
    Create(id ID, parentID ID, roUser map[string][]byte, rwUser map[string][]byte) (Executor, error)

    // Should provide options to control whether this returns:
    //  * data - should allow caller to provide checksum and only return if current checksum differs
    //         - should not return binary data by default
    //  * rwUser/roUser/attributes
    //         - should allow caller to specify only portions of metadata to return
    //         - should return all user data and attributes by default
    Read(id ID, filterspec ???) (Storage, error)

    // Updates the various modifiable aspects of the element
    //  * ID must exist
    //  * can only be updated if location policy allows writing - applies to data and rwUser data
    //  * rwUser data - if the value of a key is nil it will be deleted from the store
    Update(id ID, data io.ReadCloser, checksum string, freespace int64, rwUser map[string][]byte) error

    // Deletes all aspects of this element, including the implementation assocaited metadata and user ro/rw metadata
    Delete(id ID) error
}
```

## Storage - element management errors

**Create** - ID
 * unknown location
 * invalid location - e.g. read-only
 * name collision

**Create** - ParentID
 * unknown parent - id not found
 * invalid parent - provided ID cannot be used as parent

**Create** -  Data/checksum
 * unsupported checksum algorithm
 * unsupported data format
 * invalid data - does not match provided checksum
 * data error - e.g. error from data reader
 * insufficient space

**Create** - Freespace
 * insufficient space - unable to satisfy request due to insufficient space
 * runtime error - e.g. error while extending storage

**Create** - roUser & rwUser
 * invalid entry - does not allow nil values, or key (or value) violates kv store constraints
 * runtime error - unable to persist for whatever reason
 * read only key - if a key exists in the roUser data then it must not be in the rwUser data


**Read** - ID
 * unknown location
 * id not found in location

**Create** - filterspec
 * invalid spec



**Update** - ID
 * unknown location
 * id not found in location
 * invalid location - e.g. read-only or create only

**Update** - Data/checksum
 * unsupported checksum algorithm
 * unsupported data format
 * invalid data - does not match provided checksum
 * data error - e.g. error from data reader
 * insufficient space
 * invalid state - unable to update data/freespace in current state

**Update** - Freespace
 * insufficient space - unable to satisfy request due to insufficient space
 * runtime error - e.g. error while extending storage

**Update** - roUser & rwUser
 * invalid entry - does not allow nil values, or key (or value) violates kv store constraints
 * runtime error - unable to persist for whatever reason
 * read only key - if a key exists in the roUser data then it must not be in the rwUser data


**Delete** - ID
 * unknown location
 * id not found in location
 * invalid location - e.g. read-only or create only

**Delete** - Invalid states
 * storage in use - could provide list of referencing IDs (guess this would be storage IDs only)
 * permission denied (if we have sub-location permissions)


Other errors while:
 * attaching disk/mounting disk for data population
 * detaching disk
 * creation of disk directories and metadata


## Storage - events

Possible event set:
* created
* modified
* deleted
* inherited

These are possible but would require significantly more work:
* bound - in use by a container
* unbound - no longer in use


## Storage - composition

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