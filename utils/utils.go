package utils

import (
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

// MustDecShiftInt d shifted by n places as an integer. The shift is of the decimal point,
// i.e. of powers of ten, and is to the left if n is negative or to the right if n is positive.
//
// Example: i = d * 10^n.
//
// Panics if the value is not an integer after shifting.
//
// Use for converting quantites from their human representation into integer form.
func MustDecShiftInt(d sdk.Dec, n int64) sdk.Int {
	i, err := DecShiftInt(d, n)
	if err != nil {
		panic(err)
	}
	return i
}

// DecShiftInt d shifted by n places as an integer. The shift is of the decimal point,
// i.e. of powers of ten, and is to the left if n is negative or to the right if n is positive.
//
// Example: i = d * 10^n.
//
// Returns an error if the value is not an integer after shifting.
//
// Use for converting quantites from their human representation into integer form.
func DecShiftInt(d sdk.Dec, n int64) (sdk.Int, error) {
	d2 := DecShift(d, n)
	if !d2.IsInteger() {
		return sdk.ZeroInt(), fmt.Errorf("failed to convert human decimal '%v' to raw integer with precision '%v'", d, n)
	}
	return d2.TruncateInt(), nil
}

// DecShift returns d * 10^n. The result is exact unless it exceeds 18 decimals,
// in which case the result is truncated.
//
// Use for converting prices from their human representation into their machine form.
func DecShift(d sdk.Dec, n int64) sdk.Dec {
	return d.Mul(Dec10(n))
}

// Dec10 returns sdk.Dec(10^n).
func Dec10(n int64) sdk.Dec {
	if n > 0 {
		return sdk.NewDec(10).Power(uint64(n))
	}
	return sdk.NewDecWithPrec(1, -n)
}
