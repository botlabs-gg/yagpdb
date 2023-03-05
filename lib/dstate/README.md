# Dstate v3

v3 of dstate is a completely redesigned from scratch state tracker that is in no way compatible with the old one.

v1 and 2 had many issues, such as:

 - Not pluggable, there was 1 tracker that you were forced to use, couldn't swap implementations easily
 - Hard to use
    - Manual lock management
    - Which functions return references and which returns copies?
    - Which references are fine to deref and which are not?, which fields are immutable?
 - Hard to develop, very complex, a lot references, complex structs, complex locking, and so on

v3 tries to fix all of these and improve things in general by defining a interface in which there is no manual lock management for the user. All read operations are safe from returned references and objects, but write operations on them is undefined behaviour as they can reference state/cached state and have multiple readers.
Another goal of this is to enable the ability to have a remote state tracker, such as implementing a seperate gateway/worker system, as such the interface is also defined with this in mind.

The core of v3 is the interface found in interface.go and a reference implementation that is a memory state tracker can be found in inmemorytracker.

The reference tracker is a per shard tracker which will be used in production with yags until its ready for a seperated gateway/worker system, because of that it's built to be very performant with a per shard lock.

The previous versions were also built during a time where not all events had a guild id attached to them, for example messages, this meant things were a bit complicated but now every event had a guild id on it which means we no longer have to do a 2 stage locking process. 
