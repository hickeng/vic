# <Template>

The <template> component deals with ...

## <Template> - configuration


## <Template> - element management

```go
// semi structured identifier
// Includes
//  * ID - identifier within it's store
typedef ID url.URL

// Descripor for a <template> element
// should implement interface(s) required for filtering
type <Template> struct {
    id          ID
    parentID    ID                      // optional
    roBlob      map[string][]byte
    rwBlob      map[string][]byte
    auditLog    ???
}


type <Template> interface {
    // Creates a new <template> element
    //  * ID - encodes both location/store in which to create it and the name by which it's addressed
    //  * parentID -
    //  * roUser - caller specific metadata that is immutable once created
    //  * rwUser - caller specific metadata that can be updated - keys must not exist in the roUser set
    //
    Create(id ID, parentID ID, roUser map[string][]byte, rwUser map[string][]byte) (<Template>, error)

    // Should provide options to control whether this returns:
    //  * rwUser/roUser/attributes
    //         - should allow caller to specify only portions of metadata to return
    //         - should return all user data and attributes by default
    Read(id ID, filterspec ???) (<Template>, error)

    // Updates the various modifiable aspects of the element
    //  * ID must exist
    //  * can only be updated if location policy allows writing - applies to data and rwUser data
    //  * rwUser data - if the value of a key is nil it will be deleted from the store
    Update(id ID, rwUser map[string][]byte) error

    // Deletes all aspects of this element, including the implementation assocaited metadata and user ro/rw metadata
    Delete(id ID) error
}
```

## <Template> - element management errors

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
 * in use - could provide list of referencing IDs (guess this would be <template> IDs only)
 * permission denied (if we have sub-location permissions)



## <Template> - events

Possible event set:
* created
* modified
* deleted
* inherited


## <Template> - composition


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


## Questions and acceptance criteria:

