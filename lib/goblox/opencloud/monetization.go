package opencloud

import (
	"context"
	"fmt"
	"net/http"
)

// MonetizationService will handle communciation with the actions related to the API.
//
// Roblox Open Cloud API Docs: https://create.roblox.com/docs/en-us/cloud
type MonetizationService service

type CreatorStoreProductQuantity struct {
	Significand int `json:"significand"`
	Exponent    int `json:"exponent"`
}

type CreatorStoreProductPrice struct {
	CurrencyCode string                      `json:"currencyCode"`
	Quantity     CreatorStoreProductQuantity `json:"quantity"`
}

type CreatorStoreProductRestriction string

const (
	CreatorStoreProductRestrictionUnspecified                 CreatorStoreProductRestriction = "RESTRICTION_UNSPECIFIED"
	CreatorStoreProductRestrictionSoldItemRestricted          CreatorStoreProductRestriction = "SOLD_ITEM_RESTRICTED"
	CreatorStoreProductRestrictionSellerTemporarilyRestricted CreatorStoreProductRestriction = "SELLER_TEMPORARILY_RESTRICTED"
	CreatorStoreProductRestrictionSellerPermanentlyRestricted CreatorStoreProductRestriction = "SELLER_PERMANENTLY_RESTRICTED"
	CreatorStoreProductRestrictionSellerNoLongerActive        CreatorStoreProductRestriction = "SELLER_NO_LONGER_ACTIVE"
)

type CreatorStoreProduct struct {
	Path         string                           `json:"path"`
	BasePrice    CreatorStoreProductPrice         `json:"basePrice"`
	Published    bool                             `json:"published"`
	Restrictions []CreatorStoreProductRestriction `json:"restrictions"`
	Purchasable  bool                             `json:"purchasable"`
	UserSeller   bool                             `json:"userSeller"`
	ModelAssetID string                           `json:"modelAssetId"`
}

type CreatorStoreProductCreate struct {
	BasePrice     *CreatorStoreProductPrice `json:"basePrice,omitempty"`
	PurchasePrice *CreatorStoreProductPrice `json:"purchasePrice,omitempty"`
	Published     *bool                     `json:"published,omitempty"`
	ModelAssetID  *string                   `json:"modelAssetId,omitempty"`
}

// CreateCreatorStoreProduct will create a new store product on the creator store.
//
// Required scopes: creator-store-product:write
//
// Roblox Opencloud API Docs: https://create.roblox.com/docs/en-us/cloud/reference/CreatorStoreProduct#Cloud_CreateCreatorStoreProduct
//
// [POST] /cloud/v2/creator-store-products
func (s *MonetizationService) CreateCreatorStoreProduct(ctx context.Context, data CreatorStoreProductCreate) (*CreatorStoreProduct, *Response, error) {
	u := "/cloud/v2/creator-store-products"

	req, err := s.client.NewRequest(http.MethodPost, u, data)
	if err != nil {
		return nil, nil, err
	}

	product := new(CreatorStoreProduct)
	resp, err := s.client.Do(ctx, req, product)
	if err != nil {
		return nil, resp, err
	}

	return product, resp, nil
}

// GetCreatorStoreProduct will fetch information on a specificed store product.
//
// Required scopes: creator-store-product:read
//
// Roblox Opencloud API Docs: https://create.roblox.com/docs/en-us/cloud/reference/CreatorStoreProduct#Cloud_GetCreatorStoreProduct
//
// [GET] /cloud/v2/creator-store-products/{product_id}
func (s *MonetizationService) GetCreatorStoreProduct(ctx context.Context, productId string) (*CreatorStoreProduct, *Response, error) {
	u := fmt.Sprintf("/cloud/v2/creator-store-products/%s", productId)

	req, err := s.client.NewRequest(http.MethodGet, u, nil)
	if err != nil {
		return nil, nil, err
	}

	product := new(CreatorStoreProduct)
	resp, err := s.client.Do(ctx, req, product)
	if err != nil {
		return nil, resp, err
	}

	return product, resp, nil
}

type CreatorStoreProductUpdate struct {
	BasePrice     *CreatorStoreProductPrice `json:"basePrice,omitempty"`
	PurchasePrice *CreatorStoreProductPrice `json:"purchasePrice,omitempty"`
	Published     *bool                     `json:"published,omitempty"`
}

// UpdateCreatorStoreProduct will update information for a specificed store product.
//
// Required scopes: creator-store-product:write
//
// Roblox Opencloud API Docs: https://create.roblox.com/docs/en-us/cloud/reference/CreatorStoreProduct#Cloud_UpdateCreatorStoreProduct
//
// [PATCH] /cloud/v2/creator-store-products/{product_id}
func (s *MonetizationService) UpdateCreatorStoreProduct(ctx context.Context, productId string, data CreatorStoreProductUpdate) (*CreatorStoreProduct, *Response, error) {
	u := fmt.Sprintf("/cloud/v2/creator-store-products/%s", productId)

	req, err := s.client.NewRequest(http.MethodPatch, u, data)
	if err != nil {
		return nil, nil, err
	}

	product := new(CreatorStoreProduct)
	resp, err := s.client.Do(ctx, req, product)
	if err != nil {
		return nil, resp, err
	}

	return product, resp, nil
}

type SubscriptionState string

const (
	SubscriptionStateUnspecified                     SubscriptionState = "STATE_UNSPECIFIED"
	SubscriptionStateSubscribedWillRenew             SubscriptionState = "SUBSCRIBED_WILL_RENEW"
	SubscriptionStateSubscribedWillNotRenew          SubscriptionState = "SUBSCRIBED_WILL_NOT_RENEW"
	SubscriptionStateSubscribedRenewalPaymentPending SubscriptionState = "SUBSCRIBED_RENEWAL_PAYMENT_PENDING"
	SubscriptionStateExpired                         SubscriptionState = "EXPIRED"
)

type SubscriptionExpirationReason string

const (
	SubscriptionExpirationReasonUnspecified         SubscriptionExpirationReason = "EXPIRATION_REASON_UNSPECIFIED"
	SubscriptionExpirationReasonProductInactive     SubscriptionExpirationReason = "PRODUCT_INACTIVE"
	SubscriptionExpirationReasonProductDeleted      SubscriptionExpirationReason = "PRODUCT_DELETED"
	SubscriptionExpirationReasonSubscriberCancelled SubscriptionExpirationReason = "SUBSCRIBER_CANCELLED"
	SubscriptionExpirationReasonSubscriberRefunded  SubscriptionExpirationReason = "SUBSCRIBER_REFUNDED"
	SubscriptionExpirationReasonLapsed              SubscriptionExpirationReason = "LAPSED"
)

type SubscriptionExpirationDetails struct {
	Reason SubscriptionExpirationReason `json:"reason"`
}

type SubscriptionPurchasePlatform string

const (
	SubscriptionPurchasePlatformUnspecified SubscriptionPurchasePlatform = "PURCHASE_PLATFORM_UNSPECIFIED"
	SubscriptionPurchasePlatformDesktop     SubscriptionPurchasePlatform = "DESKTOP"
	SubscriptionPurchasePlatformMobile      SubscriptionPurchasePlatform = "MOBILE"
)

type SubscriptionPaymentProvider string

const (
	SubscriptionPaymentProviderUnspecified  SubscriptionPaymentProvider = "PAYMENT_PROVIDER_UNSPECIFIED"
	SubscriptionPaymentProviderStripe       SubscriptionPaymentProvider = "STRIPE"
	SubscriptionPaymentProviderApple        SubscriptionPaymentProvider = "APPLE"
	SubscriptionPaymentProviderGoogle       SubscriptionPaymentProvider = "GOOGLE"
	SubscriptionPaymentProviderRobloxCredit SubscriptionPaymentProvider = "ROBLOX_CREDIT"
)

type Subscription struct {
	Path              string                        `json:"path"`
	CreateTime        string                        `json:"createTime"`
	UpdateTime        string                        `json:"updateTime"`
	Active            bool                          `json:"active"`
	WillRenew         bool                          `json:"willRenew"`
	LastBillingTime   string                        `json:"lastBillingTime"`
	NextRenewTime     string                        `json:"nextRenewTime"`
	ExpireTime        string                        `json:"expireTime"`
	State             SubscriptionState             `json:"state"`
	ExpirationDetails SubscriptionExpirationDetails `json:"expirationDetails"`
	PurchasePlatform  SubscriptionPurchasePlatform  `json:"purchasePlatform"`
	PaymentProvider   SubscriptionPaymentProvider   `json:"paymentProvider"`
	User              string                        `json:"user"`
}

type SubscriptionView string

const (
	SubscriptionViewUnspecified SubscriptionView = "VIEW_UNSPECIFIED"
	SubscriptionViewBasic       SubscriptionView = "BASIC"
	SubscriptionViewFull        SubscriptionView = "FULL"
)

type SubscriptionOpts struct {
	View *SubscriptionView `url:"view,omitempty"`
}

// GetSubscription will fetch information on a specificed subscription.
//
// Required scopes:
//
// - universe:write
//
// - universe:writeuniverse.subscription-product.subscription:read
//
// Roblox Opencloud API Docs: https://create.roblox.com/docs/en-us/cloud/reference/Subscription#Cloud_GetSubscription
//
// [GET] /cloud/v2/universes/{universe_id}/subscription-products/{product_id}/subscriptions/{subscription_id}
func (s *MonetizationService) GetSubscription(ctx context.Context, universeId, productId, subscriptionId string, opts *SubscriptionOpts) (*Subscription, *Response, error) {
	u := fmt.Sprintf("/cloud/v2/universes/%s/subscription-products/%s/subscriptions/%s", universeId, productId, subscriptionId)

	u, err := addOpts(u, opts)
	if err != nil {
		return nil, nil, err
	}

	req, err := s.client.NewRequest(http.MethodGet, u, nil)
	if err != nil {
		return nil, nil, err
	}

	subscription := new(Subscription)
	resp, err := s.client.Do(ctx, req, subscription)
	if err != nil {
		return nil, resp, err
	}

	return subscription, resp, nil
}
