package domain

import "errors"

// Wallet errors
var (
	ErrInsufficientFunds            = errors.New("insufficient funds")
	ErrTransactionNotFound          = errors.New("transaction not found")
	ErrWalletNotFound               = errors.New("wallet not found")
	ErrDuplicateTransaction         = errors.New("duplicate transaction")
	ErrTransactionAlreadyRolledBack = errors.New("transaction already rolled back")
	ErrOriginalTxNotBet             = errors.New("original transaction is not a bet")
	ErrCurrencyMismatch             = errors.New("currency mismatch")
	ErrInvalidAmount                = errors.New("invalid amount: must be positive")
	ErrLockNotAcquired              = errors.New("wallet locked by concurrent request, retry")
)

// Operator errors
var (
	ErrOperatorNotFound    = errors.New("operator not found")
	ErrOperatorSuspended   = errors.New("operator is suspended")
	ErrOperatorInactive    = errors.New("operator is inactive")
	ErrInvalidSignature    = errors.New("invalid request signature")
	ErrExpiredTimestamp    = errors.New("request timestamp expired")
	ErrAPIKeyRequired      = errors.New("X-API-KEY header required")
	ErrSignatureRequired   = errors.New("X-SIGNATURE header required")
	ErrTimestampRequired   = errors.New("X-TIMESTAMP header required")
)

// Session errors
var (
	ErrSessionNotFound  = errors.New("session not found")
	ErrSessionExpired   = errors.New("session expired")
	ErrSessionRevoked   = errors.New("session revoked")
)
