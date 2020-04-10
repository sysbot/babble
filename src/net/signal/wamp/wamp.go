// Package wamp implements a WebRTC signaling system using RPC over WebSockets.
// This package  contains a wamp server that relays RPC requests between
// connected clients, and a client which implements the Signal interface, and
// which can be used to instantiate a WebRTCStreamLayer
package wamp

const (
	// ErrProcessingOffer indicates that the client who received the offer ran
	// into an error while processing it.
	ErrProcessingOffer = "io.babble.processing_offer"
)
