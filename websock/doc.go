/*
Package ws enhances Gorilla client websockets by handling graceful closing on
both sides using polite close control messages. This is as opposed to simply
tearing down the transport (TLS) connection.
*/
package websock
