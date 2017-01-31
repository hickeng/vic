# Internal Layout of the Portlayer

To simplify development experience the portlayer will be structured in a common fashion between components.

```
<component>/
    init/
        config.go
        extension.go
        extension_linux.go
    join/
        linux.go
        linux_directboot.go
        win.go
    bind/
        linux.go
        linux_directboot.go
        win.go

    join.go
    bind.go
    unbind.go
    leave.go

    create.go
    read.go
    update.go
    delete.go

    list.go

    diagnostics.go

    events.go

    # 
    configure.go
```
