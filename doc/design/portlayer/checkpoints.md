# Checkpoints


Checkpoint - point-in-time state, referencable at a later time.

This has been very awkward to consider, primarily because it has been conflated with pause/resume. Given pause can be cleanly phrased as rebinding with zero CPU
I'm no long going to consider pause/resume as part of the checkpoint mechanism. If _vSphere_ requires that a VM be suspended prior to taking a snapshot, that's a
different matter and can be addressed in the checkpoint implementation.


## Task aware checkpointing

Many applications in the HPC world allow for checkpointing, and others can be considered to do so. An example of the latter is using ahead-of-time compiled
classes with a JVM. All of these, that I am aware of, store the checkpoint state on disk. It's possible that they will start to use NVM, however that's more
suited to protection against unexpected failure than logical checkpointing.

Task aware checkpointing is done without needing support in the executor or loader, and as such exists cleanly within the existing run, modify, commit workflow
that's restricted purely to disk state. As such this does not need to be addressed by a port layer checkpoint solution.

A task may perform checkpointing simply by returning appropriate exit codes:
* run container A to checkpoint
* A exits with exit code X, having recorded state
* caller checks exit code, sees that it's X (checkpoint) and not Y (complete)
* caller commits A as A'
* caller runs A' and waits for exit code, repeating until exit code is Y


## Intent

If we discard task and offline storage checkpointing and from consideration, then the intent of a checkpoint can be surmised as:
* optimization of task or storage checkpoint (i.e. avoid the stop/commit/run cycle)
* reuse environment/executor/loader state - this is runtime state

Re-using runtime state directly applies the following restrictions:
* same loader (a portable format for reuse of runtime state can be considered task level)
* same/similar executor configuration

This implies a persistence mechanism (of potentially limited lifespan) that combines:
* base executor configuration
* loader
* runtime state

It makes no sense to checkpoint from the outside, without knowledge of where a process/task is in it's lifecycle. This in turn implies:
* the container triggers the checkpoint (could be solicited by specific signal/interrupt)
* a portlayer component receives the trigger and
  * performs any external operations needed to complete checkpoint
    - if any are needed, this implies portlayer awareness of the mechanism in use - implies mechanism should be identified in trigger
    - includes any prep needed to allow container to proceed past checkpoint when run
  * publishes the checkpoint so it can be reused


## Publishing checkpoints

Assuming construction, manipulation and management of runtime state is tightly coupled with the execution environment and executor, listing checkpoints
in the Execution portion of portlayer seems a better fit than with the less constrained images. This holds true whether the underlying mechanism preserves the
existing state for re-use, or reinflates state via some means.

If we use a capability mechanism, then this could be accomplished by adding a <parent> capability.

If an environment supports direct cloning without any explicit preparation (although I'm unsure how this can be done in a manner that's friendly to the running 
tasks), then every running container could be tagged as a parent.


## Triggering checkpoint

If triggered from the container this mechanism is fundamentally _loader_ specific, and the implementation is _executor_ specific.

A possible approach for Linux would be to send a defined signal to `pid 1` assuming `init` is modified to support checkpointing and relay that signal to the
execution environment by executor specific means.


## Questions
* how is a container transformed into a logic?
  - a checkpoint is created
  - that checkpoint is published as a logic
    * is there any utility in a checkpoint that's not published?
  * for a non-running container this is easy - it's purely related to disk state
  * for a running container - relates to disk, memory, network, files, et al
    * does the execution environment, executor, loader, or task support checkpointing?
    * at what level is checkpointing required?
      - it seems likely that the lower the level checkpointing occurs at, the more constrained the result is.
        - vmfork constrains the environment, executor, loader, and tasks to be identical
        - task checkpointing constrains only the task and whereever task state was persisted
        - should we allow for checkpointing volumes as well as logic?
      * how do we determine what constraints exist for a checkpoint approach?
        - do we need to, or do we let the mechanism fail at resume time?
        - an example of a constraint is whether a given datasource needs to exist, e.g. volume with checkpoint data
    * how do you chose which level to use if multiple are available?
    * how do we support guest-side checkpointing?
      - do we need to control whether a guest is _allowed_ to trigger a checkpoint?
