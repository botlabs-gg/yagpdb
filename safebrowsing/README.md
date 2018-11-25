Central proxy server for safebrowsing

Since we use the sotred local database version of the safebrowsing api, we need a central server to query on, we can't have a database on each bot process or node. (would likely run into api quota issues at some point)