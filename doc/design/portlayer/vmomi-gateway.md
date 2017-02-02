# VMOMI Gateway

This component provides the mechanism by which we interact with the infrastructure in a secure fashion.

## Configuration

The primary configuration aspects of this component are:
* credentials
* RPC target


## Services

The component provides a base set of services for code neededing to leverage the infrastructure.


### Authentication & connection management

The contract here is very simple; the gateway makes available an authenticated connection to the infrastructure VMOMI API, without the consumer needing to provide
_any_ inputs. That means that all of the following are taken care of:
* connecting or reconnecting
* authenticating
* credential management
* session management
* etc

This may include applying restrictions/validation to API arguments, particularly:
* Datastore paths
* Inventory paths


### Key/Value stores

A blob store is the minimum needed to allow for basic persistence, however we chose to provide a key/value store interaction to allow for a slightly more efficient
workflow. Whether this efficiency can be realized is dependent on the implementation. Examples:
* single file based - needs to be full downloaded/uploaded for each read/put
* system managed - individual keys may be read/put

It should be possible to create a kvstore associated with any infrastructure entity. Where the system provides no mechanism directly associated with an entity we
will need to manage the lifecycle of a sidecar style kvstore ourselves, along with associated consistency issues.

The primary entities we're interested in at this time:
* VMDK
* VM
* Port group


### Event subsystem
