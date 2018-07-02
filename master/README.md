Master server that managed slave bot instances of YAGPDB

Currently its main duty is zero downtime restarts.

Slaves and the master talks over TCP using the event ID (int32) followed by data in json format.

Handshake:

Upon connecting, the slave need to tell the master that it's a slave, after that the master will tell the slave what to do, be it a cold start or a greacefull start.

