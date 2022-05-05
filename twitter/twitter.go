package twitter

//go:generate sqlboiler --no-hooks psql

import (
	"sync"

	"github.com/botlabs-gg/yagpdb/v2/common"
	"github.com/botlabs-gg/yagpdb/v2/common/config"
	"github.com/botlabs-gg/yagpdb/v2/common/mqueue"
	"github.com/botlabs-gg/yagpdb/v2/lib/go-twitter/twitter"
	"github.com/botlabs-gg/yagpdb/v2/twitter/models"
	"github.com/dghubble/oauth1"
)

var (
	logger = common.GetPluginLogger(&Plugin{})

	//api := anaconda.NewTwitterApiWithCredentials("your-access-token", "your-access-token-secret", "your-consumer-key", "your-consumer-secret")
	confTwitterAPIAccessToken       = config.RegisterOption("yagpdb.twitter.access_token", "Twitter access token", "")
	confTwitterAPIAccessTokenSecret = config.RegisterOption("yagpdb.twitter.access_token_secret", "Twitter access token secret", "")
	confTwitterAPIConsumerKey       = config.RegisterOption("yagpdb.twitter.consumer_key", "Twitter consumer key", "")
	confTwitterAPIConsumerSecret    = config.RegisterOption("yagpdb.twitter.consumer_secret", "Twitter consumer secret", "")
)

type Plugin struct {
	Stop chan *sync.WaitGroup

	feeds      []*models.TwitterFeed
	feedsLock  sync.Mutex
	twitterAPI *twitter.Client
}

func (p *Plugin) PluginInfo() *common.PluginInfo {
	return &common.PluginInfo{
		Name:     "Twitter",
		SysName:  "twitter",
		Category: common.PluginCategoryFeeds,
	}
}

func RegisterPlugin() {
	if confTwitterAPIAccessToken.GetString() == "" || confTwitterAPIAccessTokenSecret.GetString() == "" || confTwitterAPIConsumerKey.GetString() == "" || confTwitterAPIConsumerSecret.GetString() == "" {
		logger.Warn("Not all twitter credentials provided, not enabling plugin")
		return
	}

	config := oauth1.NewConfig(confTwitterAPIConsumerKey.GetString(), confTwitterAPIConsumerSecret.GetString())
	token := oauth1.NewToken(confTwitterAPIAccessToken.GetString(), confTwitterAPIAccessTokenSecret.GetString())
	httpClient := config.Client(oauth1.NoContext, token)
	twitterClient := twitter.NewClient(httpClient)

	p := &Plugin{
		twitterAPI: twitterClient,
	}

	common.RegisterPlugin(p)
	mqueue.RegisterSource("twitter", p)
	common.InitSchemas("twitter", DBSchemas...)
	go p.CheckCredentials()
}

func (p *Plugin) CheckCredentials() {

	user, _, err := p.twitterAPI.Accounts.VerifyCredentials(&twitter.AccountVerifyParams{
		SkipStatus:   twitter.Bool(true),
		IncludeEmail: twitter.Bool(true),
	})
	if err != nil {
		logger.WithError(err).Fatal("Failed verifying credentials")
	} else {
		logger.Infof("Logged in as %s", user.ScreenName)
	}
}

const TwitterIcon = `data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAOEAAADhCAMAAAAJbSJIAAAAY1BMVEVQq/H///9ApvBHqPFEp/A8pPDs9f17vfT7/f/x+P5bsPLg7/y22fj2+/7R5/vC3/ltt/OXyvbb7Pyo0vfn8v3J4/qMxfVVrvFks/KDwfR+vvTG4fqw1viVyfal0PfO5fsroO9Jysg1AAAGvUlEQVR4nO2d2ZaiMBCGsSoBBUQWFVzQef+nHNx63CFQlTB96rtr+wj5JUktqQTPEwRBEARBEARBEARBEARBEARBEARBEARBEARBEARB+J0gqjOI6LopDCBAGWT7eLFbxEUdlKB+lUqE8JDMJ/fMdrUHv0UklMVy8o4886Dtq3aaOAioVm/lXViUXzQ2X90pey3thwqTL/rOGr33fRXPP0099o4M+xZ9JzL9+kWETX76H2lr6KdwVb0ff89E5eOtEfDgn/9zbBmnZs2ZB8QS9aGTvhObu8eIEBxvn1eEzVHFZBKSStS7zgInk+L6sFCHhf/zafKm//anueCM8nrQNsU8EsPFbD70a0pboc5TwpKu20NuJLCZU3X6bDYzSlOB0/M1IyqJxgInk+nzBzllH8X6etWERiIsjAW+QDpm7n7ynEKiyoYLnFS0U/u/C+fDOz9WBAJpbReu7y4dDb40+B/b3ZkNrUOqHoaNXw7TqLq4ai0EPwJRpwRG4+lHnw8z/SGdwMZ/q6OI4nG+3GHAfAPHd202YX6ZZBDK7OQ2ULhu6ctNsv4SXy9mSNQEAQiqOkSXphA8Qty83mbRVyJ8C3i7EP/R5WafzG4NobD7+M58LXsOxnKgwF2cz+7+JLHPHhbvbjXd9Ln4+2v1xqexip9ateiR8YNuQW9HqDy3j7+7nxo/Rgp35ofpQMv8T+HnYDw2zGl+uZQ5MyqBj07bE76ZaTSMe78L/JCF66Mw+HajJDTQqOffLmXEkkreiRY/a989DUfgsV3JSWMLaPnl51nH4YhbKoEr0iRUh9HjbzppfOs79GFPmSj1upnpaKPbNSJB4HSiJhbY0Yr561YPQMUU+qYp/XqM7haVzw74/cclUfic5Cehuze5SL8NSBKFNcuKmkFEsMzwo0iKBMbkwLKiZhbVJesPIkkiCx6FppH59Lj24NURILEWTAp7hObRPoCnR/kuWzAWhf38rekxq/C+cIQieOJS2HuSmOdxXSlodOLPCs8oFXpqUKbaT+JsU5UQDVeYcSnEwWnAhinBM1yz1V90KpywwIavwkSbL2tyELAJbDrqrP3+/FAWYLwoDAmG0WA4Oum2uhV7UMw2Q2FwvJu4dRFevJPvSSkr+OTR79VfTjZnF0yRZVr6QlYPcq/wksad7uoQQKeOxyJHweVdROCv9nXtVuKeI8KniAjIYCkpJV1OGUrKINDzxmAGb7AIpCiAoYLDWDQmwqQUlBnSsuAfyJLxBBQ8kcXQ8gJCqGuxr2jS1fdBMAWHxBUUA+Dw2c6MppvGXJtkNOH6+yDYUhgjiJou/PocDWn1+iOjiO4Zs8Ge4RYXNjizUJ5HVwrTGx6n9MYYJps974ZKcG/2eTtpI9H1UFzyzaQ3iY5NBudMepPo1rWxsXObYktWb4i2lbVJdBgMM4WGz3TdnkzPjH2euWGwQ5kUC/PMDeWRFOAZMrWm7wSUe+spVLbY9wMK1paNo119J1CX2dGeN957o5UxYak1XCm3hTWJ1s5pwWwyX+bH4zGJfJtD0d4jfN1taQd7+gi2gPaBOTB8wEkgPLV60I4mKL4zxe5RQnT7XjoTWfNIL9hPnXInL16g257VjdiipbhAsqegOzMHJ5bRbuZtw1Lg+4jNxTaSDffGWJxPfUen6kHd3jYaUlen6oGlVMbB3cGI2krSzU4C8QNgoZ7PhaG4Q6Xshe3Ep10Zg9wZcMadFV2BgLOm7+ByEN5AyNgyNW5M/Suoah4fzuk0+ghCGtPPOWzlXf1QujocSVUu3U8yzyDoMl3XRNEx7bGPdOAfomIG37WSD6BH9ATHKhDWREnwEY7BE+hRpYqHH67JAWqyvUIjsoN36IAsTWw/sdYBqOjWMgYcOckGVHTFYBxn6wwE9deXNxjCcrbOIBSuKZdp4pEEE1dQQbqgXAWeDznYlhwEFRDHE8exvCAIm2enqkNCvIY/Xbvqoal3qry4vIbpVH0RBtkip4/qV3SHPJoS+JNlnuwWDaskYsqtzbYOpxi0UNy9d/zyKuXxpgxXpXsjDxVf7XOejsMGQsWzdB8F49B3QqeEztmVfNvhtEyLQBiT2sDd1wMW3aAwo8rhz4q2d8Y5AiGg6KzHbgfWugEB18Nm1nz87zVE8NarnkMyycpxzS6fQIC0iAxVLuOXgyLHDWovKJKOnmq+33r/45tFEUF7m8Mumn18nPPlsViH+n9U90MTMGrlVUFdxE34ked51NCEI/Gh3lae0m/Oaf0/wdPLfOEfv/WtvoIgCIIgCIIgCIIgCIIgCIIgCIIgCIIgCIIgCG75C/S/Yj4/JsekAAAAAElFTkSuQmCC`

func (p *Plugin) WebhookAvatar() string {
	return TwitterIcon
}
