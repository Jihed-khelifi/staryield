package payment

import "errors"

// ErrConfiguration means the processor refused the platform's credentials
// (revoked or wrong API key, missing permission): a deployment problem the
// customer cannot fix. Adapters log it as a configuration error; callers
// refuse the payment instead of treating the outcome as unknown (revisions
// 6.0, §26.5).
var ErrConfiguration = errors.New("payment: processor configuration error")
