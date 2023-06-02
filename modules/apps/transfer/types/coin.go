package types

import (
	"fmt"
	"strings"

	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

// SenderChainIsSource returns false if the denomination originally came
// from the receiving chain and true otherwise.
func SenderChainIsSource(sourcePort, sourceChannel, denom string) bool {
	// This is the prefix that would have been prefixed to the denomination
	// on sender chain IF and only if the token originally came from the
	// receiving chain.

	return !ReceiverChainIsSource(sourcePort, sourceChannel, denom)
}

// ReceiverChainIsSource returns true if the denomination originally came
// from the receiving chain and false otherwise.
func ReceiverChainIsSource(sourcePort, sourceChannel, denom string) bool {
	// The prefix passed in should contain the SourcePort and SourceChannel.
	// If  the receiver chain originally sent the token to the sender chain
	// the denom will have the sender's SourcePort and SourceChannel as the
	// prefix.

	voucherPrefix := GetDenomPrefix(sourcePort, sourceChannel)
	return strings.HasPrefix(denom, voucherPrefix)
}

// GetDenomPrefix returns the receiving denomination prefix
func GetDenomPrefix(portID, channelID string) string {
	return fmt.Sprintf("%s/%s/", portID, channelID)
}

// GetPrefixedDenom returns the denomination with the portID and channelID prefixed
func GetPrefixedDenom(portID, channelID, baseDenom string) string {
	return fmt.Sprintf("%s/%s/%s", portID, channelID, baseDenom)
}

// GetIBCDenom generates the full IBC denomination string based on
// the provided source portID, source channelID, and baseDenom.
func GetIBCDenom(sourcePortID, sourceChannelID, baseDenom string) string {
	denomTrace := ParseDenomTrace(GetPrefixedDenom(sourcePortID, sourceChannelID, baseDenom))
	return denomTrace.IBCDenom()
}

// GetTransferCoin creates a transfer coin with the source port ID and channel ID
// prefixed to the base denom.
func GetTransferCoin(sourcePortID, sourceChannelID, baseDenom string, amount math.Int) sdk.Coin {
	return sdk.NewCoin(GetIBCDenom(sourcePortID, sourceChannelID, baseDenom), amount)
}
