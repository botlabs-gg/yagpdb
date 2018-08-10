Master server that managed slave bot instances of YAGPDB

Currently its main duty is zero downtime restarts.

Slaves and the master talks over TCP using the event ID (uint32) followed by data in msgpack format.

Handshake:

Upon connecting, the slave need to tell the master that it's a slave, after that the master will tell the slave what to do, be it a cold start or a greacefull start.

Currently only 1 main slave and 1 slave being migrated to is supported, in the future i may add the ability to manage multiple slaves with their own sets of shards, on multiple machines.

