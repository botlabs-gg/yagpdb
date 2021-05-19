# Pluggable state interface

Problems with the current one (dstate):

 - Not pluggable, have to dstate
 - Hard to use
    - Manual lock management
    - Which functions return references and which returns copies?
    - Which references are fine to deref and which are not?, which fields are immutable?
 - Hard to develop, very complex, a lot references, complex structs, complex locking, and so on