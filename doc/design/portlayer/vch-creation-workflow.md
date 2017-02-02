# VCH Creation Workflow

This is a worked example of VCH creation, via self-hosted portlayer, to validate & illustrate portlayer APIs.

* confirm execution environment configuration

* create/adopt top-level resource pool (vApp)
  * with limits
  * with customer-provided extraconfig values
* create/adopt storage locations
  - Locations for:
    * infrastructure (used to run endpoint)
    * images
    * volumes
  - Workflow:
    * ensure directory is empty
    * create policy file
        * shared (true/false)
        * owner (VCH id)
        * create/read-only/read-write
    * confirm ownership in policy file
* create/adopt bridge network fabric (e.g. DPG)
  * don't know where ownership is recorded
  

* upload loaders
  * direct boot
    * with associated extra VMX options needed
  * bootstrap
  * vmfork

* create vmfork template
  * vNIC hot add, or on bridge network?
  * should this be created by vic-machine or endpoint?
  * tagged as <parent> and <hidden>
  * how do we configure endpoint to use this as parent?
    - should it be a failure if this isn't available for use?

* create endpoint
  * loader: direct boot
  * networks: management, client, public, & bridge
    * configure IPs as needed
  * logic: vice/endpoint
    * configure mount point as /
  * volumes: log persistence (mounted noexec/squashed/write-only)
  * tasks: personality, portlayer, vicadmin
    * configure for filesystem logging

* wait for endpoint to initialize
  * wait for started events for each task? for the container as a whole?



## Call-flow detail

Configure vmPL (vic-machine portlayer) with:
* compute - VCH pool
* storage - infrastructure location only (raw packaging)
* storage - infrastructure location only (vmdk packaging)
* network - management, client, public, & bridge


```go
// create the direct boot loader
// NOTE: must provide the direct boot extra VMX options
s1, err := storage.Create(<infra-raw/direct-boot>, "", <tar of direct boot pieces>, checksum, 0, <capabilities>, <requirements>, <rometa>, <rwmeta>)
// create the rootfs image with endpoint binaries
s2, err := storage.Create(<infra/endpoint-image>, "", <Reader to appliance image>), checksum, 0, <rometa>, <rwmeta>)
// create a non-persistent volume for tmp image download
s3, err := storage.Create(<infra/endpoint-tmp>, "", nil, "", <freespace for image download>, <rometa>, <rwmeta>)
// create a persistent volume for log persistence
s4, err := storage.Create(<infa/endping-logs>, "", nil, "", <freespace for logs>, <rometa>, <rwmeta>)
```

```go
h := <how do I get a handle?>

// Being used as a loader may constrain how s1 presents, so this is Join rather than Bind
// should ensure that a storage adapter appropriate for the loader is configured
h = storage.Join(h, s1.ID, <loader>)
h = storage.Bind(h, s1.ID)
h = storage.Join(h, s2.ID, "/")
h = storage.Join(h, s3.ID, <non-persistent>)
h = storage.Bind(h, s3.ID, "/tmp/images/,<read-write>")
h = storage.Join(h, s4.ID, <persistent>)
h = storage.Bind(h, s4.ID, "/var/log/vic,<write-only>")
```


Networks are configured (as with --container-network) so do not need creating. If using NSX instead of pre-existing
networks then we may well have a network.Create section - that would specify items such as address ranges, DNS, network wide
pre-configured aliases, etc.

```go
// should ensure that the NIC type is appropriate for the loader
h = network.Join(h, <loc/xyz>) # interface name(net name), names on net (this), aliases on net (others)
// consider supporting joining the same ID multiple times with different net names
// contingent on support for this based on loader & tether
h = network.Join(h, <loc/xyz>) # interface name(net name), names on net (this), aliases on net (others)

// TODO: how do we express microsegmentation directives?
```

```go
// 
e1 = exec.Create(<infra/endpoint>, "", )
```

```go
h = exec.Join(h, e1.ID)
h = exec.Bind(h, e1.ID, cpu, memory)
```

```go
// does it make sense to allow for creating a task from a parent? in-guest clone?
// env byte[]?
t1 := task.Create(<id>, <parentID>, env byte[], /path/to/personality, args[])
t2 := task.Create(<id>, "", env byte[], /path/to/portlayer, args[])
t3 := task.Create(<id>, "", env byte[], /path/to/vicadmin, args[])
```

```go
h = task.Join(h, t1)
// requires logging, and will try restart (if the task comes up and remains up for 10s - purely example)
// should credentials be here or in task?
// should executor stop/restart based on it's policy - triggered by "required" processes failing restart?
h = task.Bind(h, t1, <logging><restart=10s><required>, "/working/dir",credentials)

h = task.Join(h, t2)
h = task.Bind(h, t2, <logging><restart=yes>, "/working/dir",credentials)

h = task.Join(h, t3)
h = task.Bind(h, t3, <logging>, "/working/dir",credentials)
```





## Specific deployment example


### Resources to assign to VCH:

Target resource pool: /clusterA/poolB/

Management network:   networkA
Public network:       vmNet
Bridge network:       dpg-bridgeNet
Client network:       vmNet
Container network:    vmNet

Volume store:         datastore1/volumes
Volume store:         nfs.server.com/volumes

Image store:          datastore2/images
Infra store:          datastore2/infra



Steps:
* Create/adopt volume store at datastore1/volumes
* Create/adopt volume store at nfs.server.com/volumes


















# Accessing VMOMI inside the portlayer