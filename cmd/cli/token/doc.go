/*
Package token provides the global “--token” CLI flag for specifying a bearer
token to authenticate with a capture service.

The flag is automatically added to the root command as a persistent flag. Client
providers get the token for the command being executed by calling [Get]:

	bearer := token.Get(cmd)

An empty token means to not use bearer token authentication.
*/
package token
