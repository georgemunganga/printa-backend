# Printa Subscription Checkout Design Notes

Printa subscriptions use Lenco Collections v2 as a server-side mobile-money collection service. A browser never provides a plan price, currency, payment reference, provider secret, or public payment key. The billing service records a pending checkout from the `vendor_tiers` catalogue before it can request payment.

## Confirmed provider workflow

Lenco’s current v2 documentation exposes mobile-money collection through `POST /collections/mobile-money`; it requires the amount, the unique reference, a mobile-money phone number, and an operator. Zambia supports the `airtel`, `mtn`, and `zamtel` operator values. Lenco reports `pay-offline` while the payer must approve the request on their phone, then recommends waiting for a webhook or re-querying the collection by reference.[1]

| Stage | Printa responsibility | Trust boundary |
| --- | --- | --- |
| Create checkout | Lock the vendor, tier, `ZMW` amount, unique reference, and expiry in PostgreSQL. | Server only |
| Collect mobile-money details | Send only an operator and phone number to Printa. | Authenticated vendor browser |
| Initiate collection | Use the database-locked amount and reference with the server-held provider secret. | Server only |
| Await approval | Show a pending state while the vendor authorizes the prompt on their phone. | Browser may poll Printa only |
| Reconcile payment | Re-query `/collections/status/:reference`; match reference, amount, currency, and successful provider status before activation. | Server only |
| Activate subscription | In one database transaction, update the subscription and create the paid invoice. | Server only |

> A provider redirect, a browser success indication, an unsigned callback, or a collection with a mismatched amount can **never** activate a subscription.

The provider’s status endpoint returns the collection identifier, reference, amount, currency, and a status that can be `pending`, `successful`, `failed`, `pay-offline`, or `3ds-auth-required`.[2] Printa treats only `successful` as payment value. `failed` is recorded as failed; every other state remains pending until a later verified result or expiry.

Webhook events can shorten the confirmation path but cannot be the sole source of truth. Lenco documents signed webhooks and advises re-querying when events can be missed.[3] Printa therefore verifies every callback by retrieving the collection again with the server-held secret. The portal also exposes a vendor-initiated status check while the payment sheet is open; no continuously running browser polling is required after the vendor leaves the page.

## Checkout presentation requirements

The existing subscription catalogue stays visually intact. A plan’s primary action opens a small in-context mobile-money payment sheet rather than a third-party hosted widget. The sheet displays the server-returned plan amount and accepts only a Zambian mobile-money number and operator. It uses the product wording **“Printa transaction charges”** where fee language is ever needed; it does not identify or promote the underlying provider.

## References

[1]: https://lenco-api.readme.io/v2.0/reference/initiate-collection-from-mobile-money "Lenco API — initiate mobile-money collection"
[2]: https://lenco-api.readme.io/v2.0/reference/get-collection-by-reference "Lenco API — get collection by reference"
[3]: https://lenco-api.readme.io/v2.0/reference/webhooks "Lenco API — webhooks"
