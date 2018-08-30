package premium

import (
	"context"
	"github.com/sirupsen/logrus"
	"time"
)

func Run() {
	ticker := time.NewTicker(time.Minute)
	time.Sleep(time.Second * 3)
	err := updateAllPremiumSlots()
	if err != nil {
		logrus.WithError(err).Error("Failed updating premium slots")
	}

	for {
		<-ticker.C
		err := updateAllPremiumSlots()
		if err != nil {
			logrus.WithError(err).Error("Failed updating premium slots")
		}
	}
}

// Updates ALL premiun slots from ALL sources
func updateAllPremiumSlots() error {
	// key: userid, value: slots
	newUserSlots := make(map[int64][]*PremiumSlot)

	for _, v := range PremiumSources {
		allUserSlots, err := v.AllUserSlots(context.Background())
		if err != nil {
			return err
		}

		// Add all slots to the total
		for userID, slots := range allUserSlots {
			if current, ok := newUserSlots[userID]; ok {
				newUserSlots[userID] = append(current, slots...)
			} else {
				newUserSlots[userID] = slots
			}
		}
	}

	// Get all current premium users
	currentPremiumUsers, err := AllPremiumUsers()
	if err != nil {
		return err
	}

	// Check for new users not present in the tracked premium users set
	newUsers := make([]int64, 0)

OUTER:
	for newUser, _ := range newUserSlots {
		for _, existingUser := range currentPremiumUsers {
			if newUser == existingUser {
				continue OUTER
			}
		}

		newUsers = append(newUsers, newUser)
	}

	// Update already tracked users, also removing expired ones
	for _, user := range currentPremiumUsers {
		premiumSlots := newUserSlots[user]
		err = updatePremiumUser(user, premiumSlots)
		if err != nil {
			return err
		}
	}

	// Update new users
	for _, user := range newUsers {
		premiumSlots := newUserSlots[user]
		err = updatePremiumUser(user, premiumSlots)
		if err != nil {
			return err
		}
	}

	return nil
}

func updatePremiumUser(userID int64, slots []*PremiumSlot) error {
	// TODO
	return nil
}
