# Twitter feeds

Twitter feeds use a scraper because elonmusk closed access to the API. 

It is advised to use a proxy for it if you're going to use it for anything more than 10 accounts
otherwise you might get ratelimited from twitter. 

There is some optional configuration, but the twitter feed should otherwise work without any added config due to defaults set in code. 


 ##  Optional configuration 

`YAGPDB_TWITTER_PROXY` accepts an http(s) or a sock proxy url, and all requests are sent through it instead of directly hitting twitter, this helps in preventing twitter from blocking your ip. If this isn't set, all requests are direct requests. 

`YAGPDB_TWITTER_BATCH_SIZE` , this is the batch size to split querying to twitter, if you have 100 twitter feeds added, and set this value to 3, then the bot will check for 3 accounts per request. 

`YAGPDB_TWITTER_POLL_FREQUENCY`, Since YAGPDB users scraping, this is the frequency between the total number of records. If you have 100 feeds, and a batch of 3, after all the feeds are polled once, the amount given here in **Minutes** will be awaited, before restarting polling. 

`YAGPDB_TWITTER_BATCH_DELAY`, this is the delay in **Seconds** between each batch. 