package policy

const (
	NoAnyDenied             = 10010
	NoncePolicyDenied       = 10011
	Erc20Erc721PolicyDenied = 10012
	OpenSourceDenied        = 10013
	ByteCodeLengthDenied    = 10014
)

var DeniedMap = map[int64]string{
	NoncePolicyDenied:       "denied by contract's creator nonce is more than threshold",
	Erc20Erc721PolicyDenied: "denied by contract is erc20 or erc721",
	OpenSourceDenied:        "denied by contract is open source",
	ByteCodeLengthDenied:    "denied by contract bytecode length is less than 500",
	NoAnyDenied:             "not match any denied policy",
}
