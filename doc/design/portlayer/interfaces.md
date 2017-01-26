# Component interface

```go
type Component interface {
    List(filterspec interface{})

    Event

    Diagnostics
}

type Event interface {

}

type Diagnostics {

}
```


# Storage

The storage component handles both volumes and images, using the same API. We do not distinguish between usage in that fashion.

## Storage - configuration

### Locations

These are storage locations such as the volume stores and image stores that we currently provide. These locations provide the following:
* organisation mechanism
* policy application
  * IO performance
  * Permissions (create, read, write)


## Storage - element management

```go
// semi structured identifier
// Includes
//  * location - e.g. volume store or image store
//  * ID - identifier within it's store
typedef ID url.URL

// Descripor for a storage element
type Storage struct {
    id          ID
    parentID    ID
    checksum    string
    size        int64
    roBlob      map[string][]byte
    rwBlob      map[string][]byte
    data        io.ReadCloser
    auditLog    ???
}

type Storage interface {
    // Creates a new storage element
    //  * ID - encodes both location/store in which to create it and the name by which it's addressed
    //  * parentID - the parent to inherit from. May be nil. The implementation will generate an error if the parent is not viable
    Create(id ID, parentID ID, data io.ReadCloser, checksum string, freespace int64, roUser map[string][]byte, rwUser map[string][]byte) (Storage, error)
    // Should provide options to control whether this returns:
    //  * data - should allow caller to provide checksum and only return if current checksum differs
    //  * rwUser
    //  * roUser
    Read(id ID) Storage
    // Updates the various modifiable aspects of the element
    //  * ID must exist
    //  * can only be updated if location policy allows writing
    //  * rwUser data - 
    Update(id ID, data io.ReadCloser, checksum string, freespace int64, rwUser map[string][]byte)
    // Deletes all aspects of this element, including the implementation assocaited metadata and user ro/rw metadata
    Delete(id ID)
}
```

## Storage - element management errors

### Create

ID
 * unknown location
 * invalid location - e.g. read-only
 * name collision

Parent
 * unknown parent - id not found
 * invalid parent - provided ID cannot be used as parent

Data/checksum
 * invalid data - does not match provided checksum
 * data error - e.g. error from data reader
 * insufficient space

Freespace
 * insufficient space - unable to satisfy request due to insufficient space
 * runtime error - e.g. error while extending storage

roUser & rwUser
 * invalid entry - does not allow nil keys
 * runtime error - unable to persist for whatever reason

Other errors while:
 * attaching disk/mounting disk for data population
 * detaching disk
 * creation of disk directories and metadata

### Read

### Update

### Delete


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
type Composition interface {
    Join(handle interface{}, id ID)
    // TODO: what does Bind do for storage elements?
    Bind(handle interface{})
    Unbind(handle interface{})
    Leave(handle interface{}, id ID)
}
```

### Composition errors