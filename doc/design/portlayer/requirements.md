# Portlayer Requirements

## Testing

To test the port layer it must allow the following within a single non-virtualized environment:
* test specific configuration
  - implies that we cannot use guestinfo directly
* end-to-end function
  - implies that we must have an interface & mocks for all OS interaction


## vic-machine

## Configuration

## Distributed use

## Versioning

## Diagnostics

What needs to be available via the API?
What are the requirements we have on logging?

## Events

## Listing and filtering

## Call cancellation

If we only provide for cancellation and not timeouts:
* do we need a mechanism to list running operations?
* cancel a running operation?