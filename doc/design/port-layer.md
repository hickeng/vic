# Port Layer

## Concepts

The port layer is a set of components that, between them, provide operational primitives supporting container style interaction. These components are:
* Storage
* Execution
* Network
* Interaction

It's important to note two design constraints that significantly impact the design and implementation of the portlayer:

1. colocation - while these components _may_ be colocated, whether in the same process or the same OS, that's not a viable assumption in the longer term.
2. mutliplicity - it's intended that the interfaces to these components permit multiple VCHs to use the same instance of a port layer component

With those constraints stated, it's worth noting that initial implementation will have the components both colocated _and_ serving only a single VCH per instance.
 

## Components


### Storage


### Execution



### Network



### Interaction


## Communication & Usage

The portlayer is intended to be consumed via a REST API, with no assumptions of there being a backchannel between components beyond vSphere itself.
 
To support this we present a _mutate handle/commit handle_ based interaction model, where the important aspect from the perspective of calling code is the possession of the handle and not its concrete value. The usage of _handles_ follows an immutable reference pattern for which there are several good analogies:

1. [realloc in C](http://linux.die.net/man/3/realloc) - the pointer returned _might_ be the same as the original, but it cannot be assumed.
2. [append in Go](https://golang.org/pkg/builtin/#append) - the runtime _might_ be able to make changes in place, but the caller cannot assume it, so must update their reference.

This interaction model prevents the calling code from needing to know which portlayer components are colocated and which are distributed while still permitted the implementation to use optimization in the case of colocation. The specific value of the handle should be considered unknown as its meaningful only within the port layer implementation, which is aware of the component topology at a given point in time.

This is similar in style to the old school [handles in the Win32 APIs](https://en.wikibooks.org/wiki/Windows_Programming/Handles_and_Data_Types#HANDLE) if another analogy is useful.

This pseudocode example shows the workflow of adding an existing :

```
func main() {
    // using string identifier in the psuedo code...
    AddContainer("cookie_monster", "gibson_network")
}

func AddContainerToNetwork(name, network string) error {
    // the handle type is entirely opaque to calling code
    var hcontainer interface{}
    var hnetwork interface{}
    var err error

    // get a handle to container with specific name
    hcontainer, err = execution.GetContainer(name)
    if err != nil {
        // likely unknown container
        return err
    }

    // get handle to the specified network
    hnetwork, err = network.GetNetwork(network)
    if err != nil {
        // likely unknown network
        return err
    }

    // add the container
    // note that for every operation that alters the container we get a handle returned
    hcontainer, err = network.Join(hcontainer, hnetwork)
    if err != nil {
        // possible policy error preventing this container on this network
        return err
    }

    // there are no assumptions made that the result of 
    hcontainer, err = network.Bind(hcontainer)
    if err != nil {
        // out of IPs perhaps
        return err
    }

    // commit the changes that we've been making
    err = execution.Commit(hcontainer)
    
    return err
}
```

