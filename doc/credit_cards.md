# Test Visa Credit Cards

Standard Luhn-valid test card numbers used across payment sandboxes (PayPal, Stripe, PayPhone). No real funds or accounts behind them — safe for local checkout testing only.

| Card Number         | Expiration        | CVV           |
|----------------------|--------------------|----------------|
| 4111 1111 1111 1111 | any future date    | any 3 digits  |
| 4012 8888 8888 1881 | any future date    | any 3 digits  |
| 4000 0566 5566 5556 | any future date    | any 3 digits  |
| 4242 4242 4242 4242 | any future date    | any 3 digits  |

## Note

This repo's configured payment strategy is PayPhone (`business_parameters.country.payment_strategy`). If checkout fails with these generic numbers, check PayPhone's own sandbox docs for dedicated test cards — they may not accept generic Visa test numbers.
